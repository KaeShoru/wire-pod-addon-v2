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
		if size < 0 || size > len(data)-offset {
			// WAV written to a non-seekable pipe cannot have its final data
			// length patched into the header. FFmpeg represents that as
			// 0xffffffff; in that case the data block is the remaining input.
			if name != "data" {
				return nil, 0, errors.New("invalid WAV chunk length")
			}
			size = len(data) - offset
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

// trackPitch estimates the fundamental frequency of 16 kHz speech every
// 30 ms (15 ms hop) using normalised autocorrelation, the same approach the
// Krotos Dehumaniser documents. It returns one value per hop; 0 marks
// unvoiced or silent frames, where the carrier should fall back to noise.
func trackPitch(samples []int16, rate int) []float64 {
	frame := rate * 30 / 1000
	hop := frame / 2
	minLag := rate / 400
	maxLag := rate / 70
	if len(samples) < frame || maxLag >= frame {
		return nil
	}
	var contour []float64
	for start := 0; start+frame <= len(samples); start += hop {
		var energy float64
		for i := 0; i < frame; i++ {
			v := float64(samples[start+i])
			energy += v * v
		}
		if math.Sqrt(energy/float64(frame)) < 500 { // silence
			contour = append(contour, 0)
			continue
		}
		bestLag, bestValue := 0, 0.35 // voicing threshold
		for lag := minLag; lag <= maxLag; lag++ {
			var sum, shifted float64
			for i := 0; i+lag < frame; i++ {
				a := float64(samples[start+i])
				b := float64(samples[start+i+lag])
				sum += a * b
				shifted += b * b
			}
			value := sum / math.Sqrt(energy*shifted+1e-9)
			if value > bestValue {
				bestValue, bestLag = value, lag
			}
		}
		if bestLag > 0 {
			contour = append(contour, float64(rate)/float64(bestLag))
		} else {
			contour = append(contour, 0)
		}
	}
	// Median-of-3 smoothing kills single-frame pitch jumps without lagging
	// behind the melody.
	for i := 1; i < len(contour)-1; i++ {
		a, b, c := contour[i-1], contour[i], contour[i+1]
		if a > b {
			a, b = b, a
		}
		if b > c {
			b, c = c, b
		}
		if a > b {
			a, b = b, a
		}
		contour[i] = b
	}
	return contour
}

// vectorVocodePCM approximates Vector's Krotos_Vocoder_12 soundbank preset:
// a harmonic carrier that follows the tracked speech pitch (the Dehumaniser
// tracks pitch every 30 ms and reshapes the spectrum), fast 5 ms envelopes,
// a small dry component and a -2 dB peak ceiling. carrierHz is only the
// fallback for fully unvoiced input. Deterministic and cheap enough for
// weak Home Assistant hosts.
func vectorVocodePCM(input []int16, carrierHz, wetMix float64) []int16 {
	if len(input) == 0 {
		return nil
	}
	carrierHz = math.Max(100, math.Min(260, carrierHz))
	wetMix = math.Max(0, math.Min(1, wetMix))
	if wetMix == 0 {
		return append([]int16(nil), input...)
	}

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
	contour := trackPitch(input, vectorAudioRate)
	hop := vectorAudioRate * 15 / 1000
	phase := 0.0
	noiseState := uint32(0x56454354) // "VECT", fixed for repeatable output.
	for i, raw := range input {
		speech := float64(raw) / 32768
		// The carrier follows the tracked voice pitch so the robotic layer
		// keeps the original melody instead of flattening it into a drone.
		freq := carrierHz
		voiced := false
		if len(contour) > 0 {
			frame := i / hop
			if frame >= len(contour) {
				frame = len(contour) - 1
			}
			if contour[frame] > 0 {
				freq = math.Max(70, math.Min(400, contour[frame]))
				voiced = true
			}
		}
		phase += freq / vectorAudioRate
		phase -= math.Floor(phase)
		noiseState = 1664525*noiseState + 1013904223
		noise := float64(int32(noiseState)) / 2147483647.0
		var carrier float64
		if voiced {
			saw := 2*phase - 1
			pulse := -1.0
			if phase < 0.43 {
				pulse = 1
			}
			carrier = 0.68*saw + 0.17*pulse + 0.15*noise
		} else {
			// Fricatives and pauses: no fundamental, hiss through the bands.
			carrier = noise
		}
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
		// Saturate only the synthetic carrier. Applying tanh after the wet/dry
		// mix compressed the original TTS even at low mix values.
		processedWet := math.Tanh(wet[i] * wetGain)
		combined[i] = (1-wetMix)*dry + wetMix*processedWet
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
