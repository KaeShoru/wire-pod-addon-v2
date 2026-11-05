package wirepod_ttr

import (
	"encoding/binary"
	"math"
	"sort"
	"testing"
)

func testWAV(rate, channels int, samples []int16) []byte {
	dataSize := len(samples) * 2
	out := make([]byte, 44+dataSize)
	copy(out[0:], "RIFF")
	binary.LittleEndian.PutUint32(out[4:], uint32(36+dataSize))
	copy(out[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(out[16:], 16)
	binary.LittleEndian.PutUint16(out[20:], 1)
	binary.LittleEndian.PutUint16(out[22:], uint16(channels))
	binary.LittleEndian.PutUint32(out[24:], uint32(rate))
	binary.LittleEndian.PutUint32(out[28:], uint32(rate*channels*2))
	binary.LittleEndian.PutUint16(out[32:], uint16(channels*2))
	binary.LittleEndian.PutUint16(out[34:], 16)
	copy(out[36:], "data")
	binary.LittleEndian.PutUint32(out[40:], uint32(dataSize))
	for i, sample := range samples {
		binary.LittleEndian.PutUint16(out[44+i*2:], uint16(sample))
	}
	return out
}

func TestDecodePCM16WAVDownmixesStereo(t *testing.T) {
	wav := testWAV(24000, 2, []int16{1000, -1000, 3000, 1000})
	got, rate, err := decodePCM16WAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 24000 || len(got) != 2 || got[0] != 0 || got[1] != 2000 {
		t.Fatalf("rate=%d samples=%v", rate, got)
	}
}

func TestDecodePCM16StreamingWAV(t *testing.T) {
	wav := testWAV(24000, 1, []int16{1000, -2000, 3000})
	// FFmpeg uses this sentinel when writing WAV to stdout because it cannot
	// seek back and update the data length after encoding.
	binary.LittleEndian.PutUint32(wav[40:], ^uint32(0))
	got, rate, err := decodePCM16WAV(wav)
	if err != nil {
		t.Fatal(err)
	}
	if rate != 24000 || len(got) != 3 || got[0] != 1000 || got[1] != -2000 || got[2] != 3000 {
		t.Fatalf("rate=%d samples=%v", rate, got)
	}
}

func TestResamplePCM16(t *testing.T) {
	input := make([]int16, 2400)
	for i := range input {
		input[i] = int16(i % 100)
	}
	got := resamplePCM16(input, 24000, vectorAudioRate)
	if len(got) != 1600 {
		t.Fatalf("got %d samples, want 1600", len(got))
	}
}

func TestVectorVocoderIsBoundedAndChangesSignal(t *testing.T) {
	input := make([]int16, vectorAudioRate)
	for i := range input {
		seconds := float64(i) / vectorAudioRate
		envelope := 0.35 + 0.65*math.Sin(math.Pi*seconds)
		input[i] = int16(12000 * envelope * math.Sin(2*math.Pi*220*seconds))
	}
	got := vectorVocodePCM(input, 180, 0.82)
	if len(got) != len(input) {
		t.Fatalf("got %d samples, want %d", len(got), len(input))
	}
	different, nonzero := false, false
	for i, sample := range got {
		if sample != input[i] {
			different = true
		}
		if sample != 0 {
			nonzero = true
		}
		if sample > 26029 || sample < -26029 { // -2 dBFS, allowing one LSB.
			t.Fatalf("sample outside limiter ceiling: %d", sample)
		}
	}
	if !different || !nonzero {
		t.Fatalf("vocoder did not produce a transformed signal")
	}
}

func TestVectorVocoderZeroMixPreservesSignal(t *testing.T) {
	input := []int16{-24000, -12000, -1, 0, 1, 12000, 24000}
	got := vectorVocodePCM(input, 180, 0)
	if len(got) != len(input) {
		t.Fatalf("got %d samples, want %d", len(got), len(input))
	}
	for i := range input {
		if got[i] != input[i] {
			t.Fatalf("sample %d changed from %d to %d", i, input[i], got[i])
		}
	}
}

func medianPitch(samples []int16) (float64, int) {
	var voiced []float64
	for _, f := range trackPitch(samples, vectorAudioRate) {
		if f > 0 {
			voiced = append(voiced, f)
		}
	}
	if len(voiced) == 0 {
		return 0, 0
	}
	sort.Float64s(voiced)
	return voiced[len(voiced)/2], len(voiced)
}

func TestTrackPitchFindsFundamental(t *testing.T) {
	input := make([]int16, vectorAudioRate)
	for i := range input {
		seconds := float64(i) / vectorAudioRate
		v := 0.0
		for h := 1; h <= 6; h++ {
			v += math.Sin(2*math.Pi*220*float64(h)*seconds) / float64(h)
		}
		input[i] = int16(9000 * v)
	}
	median, frames := medianPitch(input)
	if frames == 0 {
		t.Fatal("no voiced frames detected")
	}
	if math.Abs(median-220) > 12 {
		t.Fatalf("tracked pitch %.1f Hz, want ~220", median)
	}
}

func TestVectorVocoderFollowsSourcePitch(t *testing.T) {
	// Voiced-like signal: harmonics of 220 Hz under a slow envelope.
	input := make([]int16, vectorAudioRate)
	for i := range input {
		seconds := float64(i) / vectorAudioRate
		envelope := 0.4 + 0.6*math.Sin(math.Pi*seconds)
		v := 0.0
		for h := 1; h <= 6; h++ {
			v += math.Sin(2*math.Pi*220*float64(h)*seconds) / float64(h)
		}
		input[i] = int16(9000 * envelope * v)
	}
	// The fallback carrier is 180 Hz; the output must instead follow the
	// 220 Hz source, like Vector's Dehumaniser does.
	got := vectorVocodePCM(input, 180, 1.0)
	median, frames := medianPitch(got)
	if frames == 0 {
		t.Fatal("no voiced frames in vocoded output")
	}
	if math.Abs(median-220) > 15 {
		t.Fatalf("vocoded pitch %.1f Hz, want ~220 (not the 180 Hz fallback)", median)
	}
}
