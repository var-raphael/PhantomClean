package cleaner

import (
	"regexp"
	"strings"
)

// splitSentences splits text into sentences on . ! ?
func splitSentences(text string) []string {
	re := regexp.MustCompile(`[.!?]+\s+`)
	parts := re.Split(text, -1)
	var sentences []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) > 10 {
			sentences = append(sentences, p)
		}
	}
	return sentences
}

// normalizeSentence lowercases and collapses whitespace for comparison
func normalizeSentence(s string) string {
	s = strings.ToLower(s)
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}

// cleanLinks removes noise links, deduplicates, and optionally strips nav links.
// Generic rules apply universally; nav rules apply when stripNavLinks is true.
func cleanLinks(links []string, stripNavLinks bool) []string {
	seen := make(map[string]bool)
	var cleaned []string

	for _, link := range links {
		// ---- Fragment stripping ----
		if strings.HasPrefix(link, "#") {
			continue
		}
		if strings.Contains(link, "/#") {
			continue
		}
		if idx := strings.Index(link, "#"); idx != -1 {
			link = link[:idx]
		}
		if link == "" {
			continue
		}

		// ---- Always strip: asset files ----
		if containsAny(link, []string{
			".css", ".js", ".map",
			".woff", ".woff2", ".ttf", ".eot",
			".ico", ".xml",
		}) {
			continue
		}

		// ---- Always strip: CDN / build pipeline paths ----
		if containsAny(link, []string{
			"/_astro/", "/_next/", "/_nuxt/", "/_gatsby/",
			"/static/assets/", "/cdn-cgi/",
			"/__webpack", "/.well-known/",
		}) {
			continue
		}

		// ---- Always strip: locale variant duplicates ----
		// e.g. /en-gb/page/, /de-de/page/ — same content, different locale
		if localePathRe.MatchString(urlPath(link)) {
			continue
		}

		// ---- Always strip: tracking & analytics params ----
		if containsAny(link, []string{
			"utm_source", "utm_medium", "utm_campaign", "utm_content", "utm_term",
			"gclid=", "fbclid=", "msclkid=", "ref=", "affiliate=",
		}) {
			continue
		}

		if stripNavLinks {
			// ---- Nav strip: social & community platforms ----
			if containsAny(link, []string{
				"x.com/", "twitter.com/", "youtube.com/", "youtu.be/",
				"facebook.com/", "linkedin.com/", "instagram.com/",
				"tiktok.com/", "pinterest.com/", "snapchat.com/",
				"discord.com/", "discord.gg/", "reddit.com/",
				"github.com/", "t.me/", "whatsapp.com/",
			}) {
				continue
			}

			// ---- Nav strip: legal / policy pages ----
			if containsAny(link, []string{
				"/privacy", "/cookie", "/terms", "/legal/",
				"/disclosure", "/trademark", "/copyright",
				"/trust-hub/", "/trust-center/", "/gdpr/",
				"/ccpa/", "/accessibility", "/responsible-disclosure",
			}) {
				continue
			}

			// ---- Nav strip: auth & account pages ----
			if containsAny(link, []string{
				"/login", "/log-in", "/signin", "/sign-in",
				"/signup", "/sign-up", "/register",
				"/logout", "/log-out", "/signout",
				"/forgot-password", "/reset-password",
				"/account/", "/dashboard/", "/profile/",
			}) {
				continue
			}

			// ---- Nav strip: support / status subdomains ----
			if containsAny(link, []string{
				"support.", "status.", "help.", "docs.",
				"community.", "forum.", "answers.",
				"feedback.", "ideas.", "uservoice.",
			}) {
				continue
			}

			// ---- Nav strip: generic footer paths ----
			if containsAny(link, []string{
				"/sitemap", "/rss", "/feed", "/press-kit",
				"/careers", "/jobs", "/about-us", "/about-overview",
				"/contact-us", "/contact/", "/advertise",
				"/investor-relations", "/press/",
				"/diversity", "/inclusion",
			}) {
				continue
			}

			// ---- Nav strip: app store links ----
			if containsAny(link, []string{
				"apps.apple.com/", "play.google.com/store",
				"appgallery.huawei.com/",
			}) {
				continue
			}
		}

		// ---- Deduplicate ----
		if seen[link] {
			continue
		}
		seen[link] = true
		cleaned = append(cleaned, link)
	}
	return cleaned
}

// containsAny checks if string contains any of the substrings
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// dedup removes duplicate strings from a slice
func dedup(items []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}
