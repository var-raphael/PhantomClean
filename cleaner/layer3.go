package cleaner

import (
	"strings"
	"unicode"
)

func Score(text string) float64 {
	if text == "" {
		return 0.0
	}

	score := 1.0
	words := strings.Fields(text)
	wordCount := len(words)

	if wordCount == 0 {
		return 0.0
	}

	if wordCount < 50 {
		score -= 0.3
	}

	specialCount := 0
	for _, r := range text {
		if !unicode.IsLetter(r) && !unicode.IsSpace(r) && !unicode.IsPunct(r) {
			specialCount++
		}
	}
	specialRatio := float64(specialCount) / float64(len(text))
	if specialRatio > 0.1 {
		score -= 0.2
	}

	totalLen := 0
	for _, w := range words {
		totalLen += len(w)
	}
	avgWordLen := float64(totalLen) / float64(wordCount)
	if avgWordLen > 15 {
		score -= 0.2
	}

	capsCount := 0
	letterCount := 0
	for _, r := range text {
		if unicode.IsLetter(r) {
			letterCount++
			if unicode.IsUpper(r) {
				capsCount++
			}
		}
	}
	if letterCount > 0 {
		capsRatio := float64(capsCount) / float64(letterCount)
		if capsRatio > 0.5 {
			score -= 0.2
		}
	}

	if score < 0 {
		return 0.0
	}
	if score > 1 {
		return 1.0
	}
	return score
}

func CountWords(text string) int {
	return len(strings.Fields(text))
}

func IsEnglish(text string) bool {
	commonWords := []string{
		"the", "is", "are", "was", "were", "and", "that",
		"this", "with", "for", "have", "has", "had", "not",
		"but", "from", "they", "been", "more", "will", "one",
	}

	lower := strings.ToLower(text)
	words := strings.Fields(lower)
	if len(words) == 0 {
		return false
	}

	matches := 0
	checkWords := words
	if len(checkWords) > 100 {
		checkWords = checkWords[:100]
	}

	for _, w := range checkWords {
		for _, common := range commonWords {
			if w == common {
				matches++
				break
			}
		}
	}

	ratio := float64(matches) / float64(len(checkWords))
	return ratio >= 0.05
}

func DetectLanguage(text string) string {
	if IsEnglish(text) {
		return "en"
	}
	return "other"
}