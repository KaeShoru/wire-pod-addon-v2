package wirepod_ttr

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
)

const vectorAudioRate = 16000

// decodePCM16WAV reads the ordinary PCM WAV returned by local TTS services.
// Mono and stereo are accepted; stereo is downmixed before processing.
func decodePCM16WAV(data []byte) ([]int16, int, error) {
	if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, 0, errors.New("TTS response is not a RIFF/WAVE file")
	}
	var format, channels, bits uint16
	var rate uint32
	var pcm []byte
	for offset := 12; offset+8 <= len(data); {
		name := string(data[offset : offset+4])
		size := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if size < 0 || offset+size > len(data) {
			return nil, 0, errors.New("invalid WAV chunk length")
		}
		switch name {
		case "fmt ":
			if size < 16 {
				return nil, 0, errors.New("invalid WAV fmt chunk")
			}
			format = binary.LittleEndian.Uint16(data[offset : offset+2])
			channels = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
			rate = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
			bits = binary.LittleEndian.Uint16(data[offset+14 : offset+16])
		case "data":
			pcm = data[offset : offset+size]
		}
		offset += size + size%2
	}
	if format != 1 || (channels != 1 && channels != 2) || bits != 16 || rate < 8000 || len(pcm) == 0 {
		return nil, 0, fmt.Errorf("unsupported WAV: format=%d channels=%d bits=%d rate=%d", format, channels, bits, rate)
	}
	frameBytes := int(channels) * 2
	frames := len(pcm) / frameBytes
	out := make([]int16, frames)
	for i := range out {
		left := int32(int16(binary.LittleEndian.Uint16(pcm[i*frameBytes:])))
		if channels == 2 {
			right := int32(int16(binary.LittleEndian.Uint16(pcm[i*frameBytes+2:])))
			left = (left + right) / 2
		}
		out[i] = int16(left)
	}
	return out, int(rate), nil
}

func resamplePCM16(input []int16, from, to int) []int16 {
	if len(input) == 0 || from <= 0 || to <= 0 || from == to {
		return append([]int16(nil), input...)
	}
	count := int(math.Round(float64(len(input)) * float64(to) / float64(from)))
	if count < 1 {
		return nil
	}
	out := make([]int16, count)
	ratio := float64(from) / float64(to)
	for i := range out {
		position := float64(i) * ratio
		lo := int(position)
		if lo >= len(input)-1 {
			out[i] = input[len(input)-1]
			continue
		}
		fraction := position - float64(lo)
		value := float64(input[lo])*(1-fraction) + float64(input[lo+1])*fraction
		out[i] = int16(math.Round(value))
	}
	return out
}

type biquad struct {
	b0, b1, b2 float64
	a1, a2     float64
	x1, x2     float64
	y1, y2     float64
}

func bandpass(sampleRate int, frequency, q float64) biquad {
	omega := 2 * math.Pi * frequency / float64(sampleRate)
	alpha := math.Sin(omega) / (2 * q)
	a0 := 1 + alpha
	return biquad{
		b0: alpha / a0,
		b2: -alpha / a0,
		a1: -2 * math.Cos(omega) / a0,
		a2: (1 - alpha) / a0,
	}
}

func (f *biquad) process(x float64) float64 {
	y := f.b0*x + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2
	f.x2, f.x1 = f.x1, x
	f.y2, f.y1 = f.y1, y
	return y
}

type vocoderBand struct {
	modulator biquad
	carrier   biquad
	envelope  float64
}

// vectorVocodePCM approximates Vector's Krotos_Vocoder_12 soundbank preset:
// a 180 Hz harmonic carrier, fast 5 ms envelopes, a small dry component and
// a -2 dB peak ceiling. It is deliberately deterministic and cheap enough for
// weak Home Assistant hosts.
func vectorVocodePCM(input []int16, carrierHz, wetMix float64) []int16 {
	if len(input) == 0 {
		return nil
	}
	carrierHz = math.Max(100, math.Min(260, carrierHz))
	wetMix = math.Max(0, math.Min(1, wetMix))

	const bandCount = 20
	const low = 140.0
	const high = 7600.0
	centers := make([]float64, bandCount)
	for i := range centers {
		centers[i] = low * math.Pow(high/low, float64(i)/float64(bandCount-1))
	}
	bands := make([]vocoderBand, bandCount)
	for i, center := range centers {
		var lower, upper float64
		if i == 0 {
			lower = low / math.Sqrt(centers[1]/low)
		} else {
			lower = math.Sqrt(centers[i-1] * center)
		}
		if i == len(centers)-1 {
			upper = math.Min(float64(vectorAudioRate)/2-50, high*math.Sqrt(high/centers[i-1]))
		} else {
			upper = math.Sqrt(center * centers[i+1])
		}
		q := math.Max(0.7, center/(upper-lower))
		bands[i].modulator = bandpass(vectorAudioRate, center, q)
		bands[i].carrier = bandpass(vectorAudioRate, center, q)
	}

	attack := math.Exp(-1 / (0.005 * vectorAudioRate))
	release := attack
	wet := make([]float64, len(input))
	phase := 0.0
	noiseState := uint32(0x56454354) // "VECT", fixed for repeatable output.
	for i, raw := range input {
		speech := float64(raw) / 32768
		phase += carrierHz / vectorAudioRate
		phase -= math.Floor(phase)
		saw := 2*phase - 1
		pulse := -1.0
		if phase < 0.43 {
			pulse = 1
		}
		noiseState = 1664525*noiseState + 1013904223
		noise := float64(int32(noiseState)) / 2147483647.0
		carrier := 0.68*saw + 0.17*pulse + 0.15*noise
		var sum float64
		for b := range bands {
			mod := math.Abs(bands[b].modulator.process(speech * 2.5119)) // original +8 dB pre-gain
			coefficient := release
			if mod > bands[b].envelope {
				coefficient = attack
			}
			bands[b].envelope = coefficient*bands[b].envelope + (1-coefficient)*mod
			sum += bands[b].carrier.process(carrier) * bands[b].envelope
		}
		wet[i] = sum
	}

	// Match wet RMS to the source before applying the wet/dry control. This
	// makes the profile stable for quiet and loud TTS engines alike.
	var dryEnergy, wetEnergy float64
	for i, sample := range input {
		dry := float64(sample) / 32768
		dryEnergy += dry * dry
		wetEnergy += wet[i] * wet[i]
	}
	wetGain := 1.0
	if wetEnergy > 1e-12 {
		wetGain = math.Sqrt(dryEnergy / wetEnergy)
	}

	combined := make([]float64, len(input))
	peak := 0.0
	for i, sample := range input {
		dry := float64(sample) / 32768
		value := (1-wetMix)*dry + wetMix*wet[i]*wetGain
		combined[i] = math.Tanh(value * 1.35)
		peak = math.Max(peak, math.Abs(combined[i]))
	}
	ceiling := math.Pow(10, -2.0/20.0)
	gain := 1.0
	if peak > ceiling {
		gain = ceiling / peak
	}
	out := make([]int16, len(input))
	for i, value := range combined {
		out[i] = int16(math.Round(value * gain * 32767))
	}
	return out
}

func pcm16Bytes(samples []int16) []byte {
	out := make([]byte, len(samples)*2)
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(out[i*2:], uint16(sample))
	}
	return out
}
