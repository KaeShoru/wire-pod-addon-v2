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
	Speaker string  `json:"speaker"`
	Profile string  `json:"profile"`
	Speed   float64 `json:"speed"`
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
	payload, err := json.Marshal(localTTSRequest{
		Text: text, Speaker: speaker, Profile: "natural",
		Speed: envFloat("VECTOR_TTS_SPEED", 0.96),
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
	volume = math.Max(10, math.Min(100, volume))
	return streamVectorPCM(robot, pcm16Bytes(samples), uint32(volume))
}
