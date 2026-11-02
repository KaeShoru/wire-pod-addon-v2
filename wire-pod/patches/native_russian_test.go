package wirepod_ttr

import (
	"strings"
	"testing"
)

func TestRussianToVectorPhonetics(t *testing.T) {
	input := "Кошка - это животное с четырьмя лапами и ушами."
	got := russianToVectorPhonetics(input)
	want := "koshka - ehto zheevotnoye s chetihrmya lapamee ee ooshamee."
	if got != want {
		t.Fatalf("unexpected phonetics:\n got: %q\nwant: %q", got, want)
	}
	if containsRussian(got) {
		t.Fatalf("phonetic output still contains Cyrillic: %q", got)
	}
}

func TestRussianToVectorPhoneticsPreservesASCII(t *testing.T) {
	input := "Vector 2.0: Wi-Fi!"
	if got := russianToVectorPhonetics(input); got != input {
		t.Fatalf("ASCII changed: got %q, want %q", got, input)
	}
}

func TestRussianToVectorPhoneticsCoversAlphabet(t *testing.T) {
	got := russianToVectorPhonetics("АБВГДЕЁЖЗИЙКЛМНОПРСТУФХЦЧШЩЪЫЬЭЮЯ")
	if containsRussian(got) || strings.TrimSpace(got) == "" {
		t.Fatalf("alphabet was not fully converted: %q", got)
	}
}
