package cleaner

import (
	"strings"
)

// StripBoilerplate removes sentences that appear across multiple files
// (frequency >= boilerplate_threshold), indicating they are boilerplate.
//
// Both the frequency update AND the filtering happen under a single lock
// to prevent concurrent goroutines from inflating frequencies mid-filter,
// which would cause legitimate sentences to be incorrectly dropped.
func (c *Cleaner) StripBoilerplate(text string) string {
	sentences := splitSentences(text)
	if len(sentences) == 0 {
		return text
	}

	c.boilerplateMu.Lock()
	defer c.boilerplateMu.Unlock()

	// Update global frequency map
	for _, s := range sentences {
		normalized := normalizeSentence(s)
		if len(normalized) > 20 {
			c.boilerplateFreq[normalized]++
		}
	}

	// Filter in the same lock — no gap for other goroutines to inflate counts
	var kept []string
	for _, s := range sentences {
		normalized := normalizeSentence(s)
		if c.boilerplateFreq[normalized] >= c.cfg.BoilerplateThreshold {
			continue // this sentence appears too often across files — drop it
		}
		kept = append(kept, s)
	}

	return strings.Join(kept, " ")
}
