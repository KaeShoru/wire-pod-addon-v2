package wirepod_ttr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/fforchino/vector-go-sdk/pkg/vector"
	"github.com/fforchino/vector-go-sdk/pkg/vectorpb"
)

type localTTSRequest struct {
	Text    string  `json:"text"`
	Engine  string  `json:"engine,omitempty"`
	Speaker string  `json:"speaker"`
	Profile string  `json:"profile"`
	Speed   float64 `json:"speed"`
	Pitch   float64 `json:"pitch"`
	Variant string  `json:"variant,omitempty"`
}

func envFloat(name string, fallback float64) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv(name)), 64)
	if err != nil {
		return fallback
	}
	return value
}

func localTTS(text string) ([]byte, error) {
	endpoint := strings.TrimSpace(os.Getenv("VECTOR_TTS_URL"))
	if endpoint == "" {
		return nil, errors.New("VECTOR_TTS_URL is empty")
	}
	speaker := strings.TrimSpace(os.Getenv("VECTOR_TTS_SPEAKER"))
	if speaker == "" {
		speaker = "eugene"
	}
	profile := strings.TrimSpace(os.Getenv("VECTOR_TTS_PROFILE"))
	if profile == "" {
		profile = "natural"
	}
	payload, err := json.Marshal(localTTSRequest{
		Text: text, Speaker: speaker, Profile: profile,
		Engine:  strings.TrimSpace(os.Getenv("VECTOR_TTS_ENGINE")),
		Variant: strings.TrimSpace(os.Getenv("VECTOR_TTS_VARIANT")),
		Speed:   envFloat("VECTOR_TTS_SPEED", 0.82),
		Pitch:   envFloat("VECTOR_TTS_PITCH", 1.0),
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("local TTS: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20<<20))
	if err != nil {
		return nil, fmt.Errorf("local TTS response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("local TTS returned %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func streamVectorPCM(robot *vector.Vector, pcm []byte, volume uint32) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	client, err := robot.Conn.ExternalAudioStreamPlayback(ctx)
	if err != nil {
		return err
	}
	if err := client.Send(&vectorpb.ExternalAudioStreamRequest{AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamPrepare{
		AudioStreamPrepare: &vectorpb.ExternalAudioStreamPrepare{AudioFrameRate: vectorAudioRate, AudioVolume: volume},
	}}); err != nil {
		return err
	}
	for offset := 0; offset < len(pcm); offset += 1024 {
		end := offset + 1024
		if end > len(pcm) {
			end = len(pcm)
		}
		chunk := pcm[offset:end]
		if err := client.Send(&vectorpb.ExternalAudioStreamRequest{AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamChunk{
			AudioStreamChunk: &vectorpb.ExternalAudioStreamChunk{AudioChunkSizeBytes: uint32(len(chunk)), AudioChunkSamples: chunk},
		}}); err != nil {
			return err
		}
		// 1024 bytes are 32 ms of 16 kHz mono PCM. Stay close to real time so
		// the robot's small streaming buffer cannot overrun on long replies.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(30 * time.Millisecond):
		}
	}
	if err := client.Send(&vectorpb.ExternalAudioStreamRequest{AudioRequestType: &vectorpb.ExternalAudioStreamRequest_AudioStreamComplete{
		AudioStreamComplete: &vectorpb.ExternalAudioStreamComplete{},
	}}); err != nil {
		return err
	}
	for {
		response, err := client.Recv()
		if err != nil {
			return err
		}
		if response.GetAudioStreamPlaybackComplete() != nil {
			return nil
		}
		if response.GetAudioStreamPlaybackFailyer() != nil {
			return errors.New("Vector rejected the audio stream")
		}
		if response.GetAudioStreamBufferOverrun() != nil {
			return errors.New("Vector audio stream buffer overrun")
		}
	}
}

// DoSayText_VectorTTS gets natural Russian speech from the configured local
// service and streams it to the robot. The experimental lightweight vocoder is
// only applied when explicitly enabled. No model is loaded on the Home
// Assistant host.
func DoSayText_VectorTTS(robot *vector.Vector, text string) error {
	wav, err := localTTS(text)
	if err != nil {
		return err
	}
	samples, rate, err := decodePCM16WAV(wav)
	if err != nil {
		return err
	}
	samples = resamplePCM16(samples, rate, vectorAudioRate)
	if strings.EqualFold(strings.TrimSpace(os.Getenv("VECTOR_TTS_VOCODER")), "true") {
		samples = vectorVocodePCM(
			samples,
			envFloat("VECTOR_TTS_CARRIER_HZ", 180),
			envFloat("VECTOR_TTS_VOCODER_MIX", 0.18),
		)
	}
	volume := math.Round(envFloat("VECTOR_TTS_VOLUME", 50))
	volume = math.Max(10, math.Min(200, volume))
	if volume > 100 {
		// Vector's SDK volume stops at 100. Values above 100 pre-amplify the PCM
		// with a soft limiter so the speaker is driven harder without harsh
		// digital clipping.
		samples = amplifyPCM16(samples, volume/100)
		volume = 100
	}
	return streamVectorPCM(robot, pcm16Bytes(samples), uint32(volume))
}

// amplifyPCM16 boosts samples above the robot's maximum SDK volume. The gain
// is strictly linear; peaks that would exceed the -1.5 dBFS ceiling are caught
// by a lookahead peak limiter (instant attack, ~100 ms release), the same
// design Vector's firmware uses after its vocoder. Loudness is maximized
// without waveshaping: a tanh knee here used to fill pauses with noise and
// smear the vocoder's harmonics, and scaling the whole utterance down to the
// highest spike made speech far too quiet.
func amplifyPCM16(samples []int16, gain float64) []int16 {
	const ceiling = 0.8414 // -1.5 dBFS
	const lookahead = 80   // 5 ms at 16 kHz
	const release = 0.9994 // ~100 ms time constant
	n := len(samples)
	boosted := make([]float64, n)
	for i, sample := range samples {
		boosted[i] = float64(sample) / 32768 * gain
	}
	// Per-sample gain: instant attack driven by the lookahead peak envelope,
	// exponential release so the limiter does not pump.
	out := make([]int16, n)
	acc := 1.0
	window := 0.0
	for i := 0; i < n; i++ {
		if i%lookahead == 0 {
			window = 0.0
			for j := i; j < i+lookahead && j < n; j++ {
				if v := math.Abs(boosted[j]); v > window {
					window = v
				}
			}
		}
		target := 1.0
		if window > ceiling {
			target = ceiling / window
		}
		if target < acc {
			acc = target
		} else {
			acc = release*acc + (1-release)*target
		}
		v := boosted[i] * acc
		if v > 1 {
			v = 1
		} else if v < -1 {
			v = -1
		}
		out[i] = int16(math.Round(v * 32767))
	}
	return out
}
