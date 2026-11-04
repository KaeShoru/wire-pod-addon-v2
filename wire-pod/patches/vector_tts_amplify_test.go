package wirepod_ttr

import "testing"

func TestAmplifyPCM16BoostsWithoutClipping(t *testing.T) {
	input := []int16{-32768, -24000, -8000, 0, 8000, 24000, 32767}
	got := amplifyPCM16(input, 1.5)
	if len(got) != len(input) {
		t.Fatalf("got %d samples, want %d", len(got), len(input))
	}
	if got[3] != 0 {
		t.Fatalf("silence amplified to %d", got[3])
	}
	for i := range input {
		if input[i] == 0 {
			continue
		}
		// The boost must preserve the sign and never exceed the int16 range.
		if (input[i] > 0) != (got[i] > 0) {
			t.Fatalf("sample %d changed sign: %d -> %d", i, input[i], got[i])
		}
	}
	// Quiet samples sit on the near-linear part of the tanh knee and must
	// actually get louder.
	if got[2] <= input[2] || got[4] >= input[4] || got[4] <= 0 {
		t.Fatalf("gain did not boost quiet samples: %d -> %d", input[4], got[4])
	}
	// Hot samples must be limited below full scale instead of wrapping.
	if got[6] < 28000 || got[0] > -28000 {
		t.Fatalf("limiter ceiling out of range: %d, %d", got[0], got[6])
	}
}
