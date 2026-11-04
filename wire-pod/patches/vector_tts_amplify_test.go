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
		// The boost must increase magnitude for quiet samples and must never
		// exceed the int16 range or invert the sign.
		if (input[i] > 0) != (got[i] > 0) {
			t.Fatalf("sample %d changed sign: %d -> %d", i, input[i], got[i])
		}
	}
	if got[5] <= input[5] {
		t.Fatalf("gain did not boost %d: got %d", input[5], got[5])
	}
	if got[6] < 32767-1 {
		t.Fatalf("full-scale sample was not limited near the ceiling: %d", got[6])
	}
}

func TestAmplifyPCM16UnityGainIsNearlyLossless(t *testing.T) {
	input := []int16{-20000, -1000, -1, 0, 1, 1000, 20000}
	got := amplifyPCM16(input, 1.0)
	for i := range input {
		diff := int(got[i]) - int(input[i])
		if diff < -2 || diff > 2 {
			t.Fatalf("sample %d drifted from %d to %d", i, input[i], got[i])
		}
	}
}
