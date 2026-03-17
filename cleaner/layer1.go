package cleaner

import (
	"bufio"
	"fmt"
	"html"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// loadPatterns reads regex pattern strings from a file (called once at startup in New())
func loadPatterns(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var patterns []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		patterns = append(patterns, line)
	}
	return patterns, scanner.Err()
}

// Strip cleans text based on content_type using pre-compiled regexes stored on Cleaner.
// "text"  → aggressive: decode entities, strip emojis, symbols, special chars
// "code"  → preserve all code syntax, only decode entities and apply patterns
// "mixed" → preserve code blocks (``` and <code>), strip emojis/symbols outside
func (c *Cleaner) Strip(text string, contentType string) string {
	// Always decode unicode escapes first: \uXXXX → actual character
	text = decodeUnicodeEscapes(text)

	// Always decode HTML entities: &amp; &lt; &gt; etc
	text = html.UnescapeString(text)

	switch contentType {
	case "code":
		text = c.applyPatterns(text)
	case "mixed":
		text = c.stripMixed(text)
	default: // "text"
		text = c.applyPatterns(text)
		text = stripEmojis(text)
		text = stripNonText(text)
	}

	// Normalize whitespace using pre-compiled regex (no recompile per call)
	text = c.whitespaceRe.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// applyPatterns applies all pre-compiled regex patterns from regex.txt
func (c *Cleaner) applyPatterns(text string) string {
	for _, re := range c.patterns {
		text = re.ReplaceAllString(text, " ")
	}
	return text
}

// stripMixed preserves code blocks (```, <pre>, <code>) then strips
// emojis/symbols from everything outside them
func (c *Cleaner) stripMixed(text string) string {
	placeholders := make(map[string]string)
	counter := 0

	// Preserve ``` backtick code blocks
	text = c.backtickRe.ReplaceAllStringFunc(text, func(block string) string {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", counter)
		placeholders[placeholder] = block
		counter++
		return placeholder
	})

	// Preserve <pre>...</pre> blocks
	text = c.preRe.ReplaceAllStringFunc(text, func(block string) string {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", counter)
		placeholders[placeholder] = block
		counter++
		return placeholder
	})

	// Preserve <code>...</code> blocks
	text = c.codeRe.ReplaceAllStringFunc(text, func(block string) string {
		placeholder := fmt.Sprintf("__CODE_BLOCK_%d__", counter)
		placeholders[placeholder] = block
		counter++
		return placeholder
	})

	// Strip non-text from remaining content
	text = c.applyPatterns(text)
	text = stripEmojis(text)
	text = stripNonText(text)

	// Restore all code blocks
	for placeholder, block := range placeholders {
		text = strings.Replace(text, placeholder, block, 1)
	}

	return text
}

// decodeUnicodeEscapes converts \uXXXX sequences to actual characters.
// Note: this uses a local compile — it's called before the Cleaner is
// fully init'd and the pattern never changes, so it's acceptable here.
// If profiling shows this is hot, move it to a package-level var.
var unicodeEscapeRe = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

func decodeUnicodeEscapes(text string) string {
	return unicodeEscapeRe.ReplaceAllStringFunc(text, func(match string) string {
		hex := match[2:]
		n, err := strconv.ParseInt(hex, 16, 32)
		if err != nil {
			return match
		}
		r := rune(n)
		// Strip null bytes and control chars except newline/tab
		if r == 0 || (r < 32 && r != '\n' && r != '\t') {
			return ""
		}
		return string(r)
	})
}

// stripEmojis replaces emoji characters with a space
func stripEmojis(text string) string {
	var b strings.Builder
	for _, r := range text {
		if isEmoji(r) {
			b.WriteRune(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// isEmoji checks if a rune is an emoji
func isEmoji(r rune) bool {
	return (r >= 0x1F300 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x27BF) ||
		(r >= 0xFE00 && r <= 0xFE0F) ||
		(r >= 0x1F900 && r <= 0x1F9FF)
}

// stripNonText removes non-text characters, keeping letters, digits,
// punctuation, and whitespace
func stripNonText(text string) string {
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) ||
			unicode.IsDigit(r) ||
			unicode.IsSpace(r) ||
			unicode.IsPunct(r) ||
			r == '\n' || r == '\t' {
			b.WriteRune(r)
		} else {
			b.WriteRune(' ')
		}
	}
	return b.String()
}
