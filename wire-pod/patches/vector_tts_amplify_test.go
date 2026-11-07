package wirepod_ttr

import (
	"math"
	"testing"
)

func TestAmplifyPCM16QuietSourceGetsFullLinearGain(t *testing.T) {
	input := make([]int16, 400)
	for i := range input {
		input[i] = int16(1000 * math.Sin(float64(i)/10))
	}
	got := amplifyPCM16(input, 2.0)
	for i := range input {
		want := int16(math.Round(float64(input[i]) * 2))
		if got[i] != want && got[i] != want+1 && got[i] != want-1 {
			t.Fatalf("sample %d: got %d, want ~%d", i, got[i], want)
		}
	}
}

func TestAmplifyPCM16LimitsSustainedLoudToCeiling(t *testing.T) {
	input := make([]int16, 4000)
	for i := range input {
		input[i] = int16(30000 * math.Sin(float64(i)/7))
	}
	got := amplifyPCM16(input, 2.0)
	peak := 0
	for _, s := range got {
		if int(s) > peak {
			peak = int(s)
		}
		if int(-s) > peak {
			peak = int(-s)
		}
	}
	// -1.5 dBFS ceiling is 27568 in int16; the lookahead limiter must keep the
	// sustained loud tone right at it, never above full scale.
	if peak > 27700 || peak < 26500 {
		t.Fatalf("limiter peak out of range: %d", peak)
	}
}

func TestAmplifyPCM16PreservesSignAndSilence(t *testing.T) {
	input := []int16{-24000, -8000, 0, 8000, 24000}
	got := amplifyPCM16(input, 1.5)
	if got[2] != 0 {
		t.Fatalf("silence amplified to %d", got[2])
	}
	for i := range input {
		if input[i] == 0 {
			continue
		}
		if (input[i] > 0) != (got[i] > 0) {
			t.Fatalf("sample %d changed sign: %d -> %d", i, input[i], got[i])
		}
	}
}
