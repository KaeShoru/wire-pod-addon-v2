package wirepod_ttr

import (
	"strings"
	"unicode"
)

// russianToVectorPhonetics converts Cyrillic to a conservative English
// phonetic spelling. Vector's native voice only understands its supported
// Latin-script languages; passing Cyrillic makes it read Unicode character
// names instead of the sentence.
func russianToVectorPhonetics(input string) string {
	var output strings.Builder
	output.Grow(len(input) * 2)

	wordStart := true
	previousRussianVowelOrSign := false

	for _, original := range input {
		r := unicode.ToLower(original)
		phoneme := ""
		isRussian := true
		isVowelOrSign := false

		switch r {
		case 'а':
			phoneme, isVowelOrSign = "a", true
		case 'б':
			phoneme = "b"
		case 'в':
			phoneme = "v"
		case 'г':
			phoneme = "g"
		case 'д':
			phoneme = "d"
		case 'е':
			if wordStart || previousRussianVowelOrSign {
				phoneme = "ye"
			} else {
				phoneme = "e"
			}
			isVowelOrSign = true
		case 'ё':
			phoneme, isVowelOrSign = "yo", true
		case 'ж':
			phoneme = "zh"
		case 'з':
			phoneme = "z"
		case 'и':
			phoneme, isVowelOrSign = "ee", true
		case 'й':
			phoneme, isVowelOrSign = "y", true
		case 'к':
			phoneme = "k"
		case 'л':
			phoneme = "l"
		case 'м':
			phoneme = "m"
		case 'н':
			phoneme = "n"
		case 'о':
			phoneme, isVowelOrSign = "o", true
		case 'п':
			phoneme = "p"
		case 'р':
			phoneme = "r"
		case 'с':
			phoneme = "s"
		case 'т':
			phoneme = "t"
		case 'у':
			phoneme, isVowelOrSign = "oo", true
		case 'ф':
			phoneme = "f"
		case 'х':
			phoneme = "kh"
		case 'ц':
			phoneme = "ts"
		case 'ч':
			phoneme = "ch"
		case 'ш':
			phoneme = "sh"
		case 'щ':
			phoneme = "shch"
		case 'ъ', 'ь':
			isVowelOrSign = true
		case 'ы':
			phoneme, isVowelOrSign = "ih", true
		case 'э':
			phoneme, isVowelOrSign = "eh", true
		case 'ю':
			phoneme, isVowelOrSign = "yoo", true
		case 'я':
			phoneme, isVowelOrSign = "ya", true
		default:
			isRussian = false
		}

		if isRussian {
			output.WriteString(phoneme)
			wordStart = false
			previousRussianVowelOrSign = isVowelOrSign
			continue
		}

		output.WriteRune(original)
		if unicode.IsSpace(original) || strings.ContainsRune(".,!?;:()[]{}\"'/-", original) {
			wordStart = true
		}
		previousRussianVowelOrSign = false
	}

	return output.String()
}

func containsRussian(input string) bool {
	for _, r := range input {
		if (r >= 'А' && r <= 'я') || r == 'Ё' || r == 'ё' {
			return true
		}
	}
	return false
}
