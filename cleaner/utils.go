package cleaner

import (
	"regexp"
	"strings"
)

// localePathRe matches locale-prefixed URL path segments.
// Covers ISO 639-1 language codes with optional region tags:
// /en/, /en-gb/, /de-de/, /zh-tw/, /pt-br/, etc.
var localePathRe = regexp.MustCompile(`^/[a-z]{2}(-[a-z]{2})?/`)

// cleanTitle strips trademark symbols and site name suffixes from page titles.
// e.g. "A Challenger in 2025 Gartner® Magic Quadrant™ | Cloudflare"
// → "A Challenger in 2025 Gartner Magic Quadrant"
func cleanTitle(title string) string {
	// Strip trademark/copyright symbols
	replacer := strings.NewReplacer(
		"®", "",
		"™", "",
		"℠", "",
		"©", "",
		"⁠", "", // zero-width non-joiner
	)
	title = replacer.Replace(title)

	// Strip site name suffix like "| Cloudflare" or "- Cloudflare"
	if idx := strings.LastIndex(title, " | "); idx != -1 {
		title = title[:idx]
	} else if idx := strings.LastIndex(title, " - "); idx != -1 {
		title = title[:idx]
	}

	return strings.TrimSpace(title)
}

// extractCDNImage unwraps Cloudflare image optimization URLs.
// e.g. https://cloudflare.com/cdn-cgi/image/format=auto/https://cf-assets...
// → https://cf-assets...
// Returns the original URL unchanged if it is not a CDN-wrapped URL.
func extractCDNImage(url string) string {
	const marker = "/cdn-cgi/image/"
	idx := strings.Index(url, marker)
	if idx == -1 {
		return url
	}
	rest := url[idx+len(marker):]
	for _, scheme := range []string{"https://", "http://"} {
		if i := strings.Index(rest, scheme); i != -1 {
			return rest[i:]
		}
	}
	return url
}

// cleanImages unwraps CDN proxy URLs and deduplicates the result.
// Drop-in replacement for dedup(page.Images).
func cleanImages(images []string) []string {
	seen := make(map[string]bool)
	var cleaned []string
	for _, img := range images {
		img = extractCDNImage(img)
		if img == "" || seen[img] {
			continue
		}
		seen[img] = true
		cleaned = append(cleaned, img)
	}
	return cleaned
}

// urlPath returns just the path component of a URL string.
// Fast extraction without net/url import.
func urlPath(link string) string {
	s := link
	if i := strings.Index(s, "://"); i != -1 {
		s = s[i+3:]
	}
	if i := strings.Index(s, "/"); i != -1 {
		return s[i:]
	}
	return "/"
}