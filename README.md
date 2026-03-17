# PhantomClean

> A robust multi-layer data cleaning pipeline for web scraped content. Rule-based cleaning, boilerplate detection, quality scoring, and a multi-provider AI cascade — all in one CLI tool.

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![License](https://img.shields.io/badge/License-BSL-green.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-var--raphael-black?logo=github)](https://github.com/var-raphael)

---

## What is PhantomClean?

PhantomClean takes raw scraped JSON files — like the ones [PhantomCrawl](https://github.com/var-raphael/PhantomCrawl) produces — and turns them into clean, structured datasets ready for AI training, analysis, or export.

It runs a 4-layer cleaning pipeline on every file, processes folders in configurable concurrent batches, watches for new files in real time, and exports results in multiple formats.

---

## How It Works

PhantomClean processes each file through 4 layers in sequence:

| Layer | What it does |
| --- | --- |
| **Layer 1** | Regex stripping — removes boilerplate patterns from `regex.txt`, strips emojis, decodes HTML entities and unicode escapes |
| **Layer 2** | Boilerplate frequency detection — tracks sentences seen across multiple files and removes ones that appear too often |
| **Layer 3** | Quality scoring — scores content on word count, special character ratio, average word length, and caps ratio. Skips files below threshold |
| **Layer 4** | AI cascade — sends content through a chain of AI providers (Groq → OpenAI → Anthropic). Falls back to rules if all fail |

Large documents are automatically chunked before being sent to AI so token limits are never hit.

---

## Input Format Requirements

PhantomClean expects scraped data to be structured as follows:

```
scraped/
  site-name/
    page-title/
      raw.json         <- required
      cleaned.json     <- optional (preferred over raw.json if present)
```

Each `raw.json` must be valid JSON and contain at least one of these text fields for PhantomClean to extract content from:

| Field | Description |
| --- | --- |
| `content` | Plain text content — checked first |
| `text` | Fallback text field |
| `html` | Raw HTML — last resort |

If none of `content`, `text`, or `html` are present, the file is skipped with reason `no text content found`.

The following fields are optional but preserved in the output if present:

| Field | Type | Description |
| --- | --- | --- |
| `url` | string | Page URL |
| `title` | string | Page title |
| `links` | array | Extracted links |
| `images` | array | Extracted image URLs |
| `emails` | array | Extracted email addresses |
| `phones` | array | Extracted phone numbers |
| `metadata.description` | string | Page meta description |
| `layer_used` | string | Which scraper layer extracted the page |
| `crawled_at` | string | When the page was crawled |

### Minimum valid `raw.json`

```json
{
  "url": "https://example.com/page",
  "title": "Page Title",
  "content": "The actual page text content goes here..."
}
```

> **Recommended scraper: [PhantomCrawl](https://github.com/var-raphael/PhantomCrawl)**
> PhantomCrawl produces exactly this structure automatically — all required and optional fields included — making it the recommended scraper for the most efficient pipeline. Using another scraper is supported as long as the JSON structure matches the format above. Mismatched structure means PhantomClean either skips the file or produces empty output.

---

## Installation

### Download Binary (Recommended)

Download the pre-built binary for your platform from [GitHub Releases](https://github.com/var-raphael/PhantomClean/releases):

```bash
# Linux (64-bit)
wget https://github.com/var-raphael/PhantomClean/releases/latest/download/phantomclean-linux-amd64
chmod +x phantomclean-linux-amd64
sudo mv phantomclean-linux-amd64 /usr/local/bin/phantomclean

# Linux ARM / Android Termux
wget https://github.com/var-raphael/PhantomClean/releases/latest/download/phantomclean-linux-arm64
chmod +x phantomclean-linux-arm64
mv phantomclean-linux-arm64 $PREFIX/bin/phantomclean

# Mac (Apple Silicon)
wget https://github.com/var-raphael/PhantomClean/releases/latest/download/phantomclean-darwin-arm64
chmod +x phantomclean-darwin-arm64
sudo mv phantomclean-darwin-arm64 /usr/local/bin/phantomclean

# Mac (Intel)
wget https://github.com/var-raphael/PhantomClean/releases/latest/download/phantomclean-darwin-amd64
chmod +x phantomclean-darwin-amd64
sudo mv phantomclean-darwin-amd64 /usr/local/bin/phantomclean

# Windows
# Download phantomclean-windows-amd64.exe from releases
# Move it to a folder in your PATH
```

Once installed, run from anywhere:

```bash
phantomclean init
phantomclean start
```

### Build From Source

Requires Go 1.21+

```bash
git clone https://github.com/var-raphael/PhantomClean.git
cd PhantomClean
go build -ldflags="-s -w" -o phantomclean .
```

---

## Quickstart

```bash
# 1. Generate config files
phantomclean init

# 2. Point folder_to_clean at your scraped data in cleaner.json

# 3. Add your AI keys to .env (optional)

# 4. Start cleaning
phantomclean start
```

Output is saved to `./organized/` as JSON (or your configured format).

---

## Commands

```bash
phantomclean init      # Generate cleaner.json, regex.txt, prompt.txt and .env template
phantomclean start     # Start cleaning — full batch scan then watch mode
phantomclean resume    # Resume an interrupted run (skips already-cleaned files)
phantomclean clean-ai  # Retry AI cleaning on files that only got rule-based cleaning
phantomclean stats     # Show cleaning statistics and file records
phantomclean reset     # Wipe all state (keeps output files)
```

---

## Configuration

Run `phantomclean init` to generate a `cleaner.json` template.

### Full Reference

```json
{
  "concurrent_folders": 3,
  "max_files": 0,
  "folder_to_clean": "./scraped",
  "output_file_name": "organized",
  "export_format": ["json"],
  "omit_folders": [],
  "watch_mode": true,
  "watch_debounce_seconds": 2,
  "overwrite": false,
  "min_word_count": 10,
  "boilerplate_threshold": 3,
  "quality_score_minimum": 0.5,
  "language": "en",
  "content_type": "text",
  "strip_nav_links": true,
  "output_folder": "./organized",
  "resume": true,
  "output": {
    "zip_on_complete": true,
    "zip_name": "dataset-{date}-{file_count}-files"
  },
  "ai": {
    "enabled": true,
    "fallback_to_rules": true,
    "only_if_no_cleaned": false,
    "prompt_file": "prompt.txt",
    "rotate": "random",
    "timeout_seconds": 15,
    "max_retries": 2,
    "providers": [
      {
        "provider": "groq",
        "model": "llama-3.3-70b-versatile",
        "keys": ["$GROQ_KEY_1", "$GROQ_KEY_2"]
      },
      {
        "provider": "openai",
        "model": "gpt-4o-mini",
        "keys": ["$OPENAI_KEY_1"]
      },
      {
        "provider": "anthropic",
        "model": "claude-haiku-4-5-20251001",
        "keys": ["$ANTHROPIC_KEY_1"]
      }
    ]
  }
}
```

### Field Reference

| Field | Default | Description |
| --- | --- | --- |
| `concurrent_folders` | `3` | Folders processed concurrently per batch |
| `max_files` | `0` | Max files to process. `0` = unlimited |
| `folder_to_clean` | `./scraped` | Root folder containing scraped data |
| `output_file_name` | `organized` | Output filename (without extension) |
| `export_format` | `["json"]` | Output formats: `json`, `csv`, `xml`, `txt`, `html`, `all_formats` |
| `omit_folders` | `[]` | Folder names to skip |
| `watch_mode` | `true` | Keep running after initial scan, watch for new files |
| `watch_debounce_seconds` | `2` | Seconds of silence before flushing buffered new files as a batch |
| `overwrite` | `false` | Re-process already-cleaned files |
| `min_word_count` | `10` | Skip files with fewer words than this |
| `boilerplate_threshold` | `3` | Sentences seen this many times across files are treated as boilerplate |
| `quality_score_minimum` | `0.5` | Skip files scoring below this (0.0–1.0) |
| `language` | `en` | Only keep English content. Use `all` to disable language filtering |
| `content_type` | `text` | `text` = aggressive cleaning, `code` = preserve syntax, `mixed` = preserve code blocks |
| `strip_nav_links` | `true` | Remove navigation links from output |
| `output_folder` | `./organized` | Where to write cleaned output |
| `resume` | `true` | Skip already-processed files on restart |
| `output.zip_on_complete` | `true` | Zip the output folder when done |
| `output.zip_name` | `dataset-{date}-{file_count}-files` | Zip filename. Supports `{date}` and `{file_count}` |

---

## AI Cleaning

PhantomClean supports a multi-provider AI cascade. It tries each provider in order and falls back to the next if one fails. If all fail, it falls back to rule-based output.

Large documents are automatically chunked (2000 words per chunk) before being sent to any provider, so token limits are never an issue.

### Setup

1. Get a free API key from [console.groq.com](https://console.groq.com)
2. Add it to your `.env` file:

```env
GROQ_KEY_1=gsk_your_key_here
GROQ_KEY_2=gsk_another_key_here

OPENAI_KEY_1=sk_your_key_here

ANTHROPIC_KEY_1=your_key_here
```

3. Reference keys in `cleaner.json` using `$VAR_NAME` — they are resolved at runtime.

Multiple keys per provider are rotated automatically (`random` or `sequential`) to spread usage across accounts.

### Supported Providers

| Provider | Models | Free Tier |
| --- | --- | --- |
| **Groq** | `llama-3.3-70b-versatile`, any Groq model | ✅ 500k tokens/day per key |
| **OpenAI** | `gpt-4o-mini`, `gpt-4o`, any OpenAI model | ❌ Paid |
| **Anthropic** | `claude-haiku-4-5-20251001`, any Claude model | ❌ Paid |

### Custom Prompt

Edit `prompt.txt` to control what the AI extracts. The default prompt instructs the model to remove boilerplate, extract core content, and return `NO_CONTENT` if nothing meaningful is found.

---

## Batch Processing

PhantomClean processes folders in strict sequential batches:

```
Found 120 folders — processing in batches of 3 (40 batches total)
  → Batch 1/40
    → site-one
    → site-two
    → site-three
  ✓ [1] site-one (groq/llama-3.3-70b-versatile) score:0.94 words:1842
  ✓ [2] site-two (rules) score:1.00 words:632
  ✓ [3] site-three (groq/llama-3.3-70b-versatile) score:0.88 words:2103
  → Batch 2/40
  ...
```

Within each batch, folders run concurrently. The next batch only starts after every folder in the current batch is done. Tune `concurrent_folders` based on your machine and API rate limits.

---

## Watch Mode

When `watch_mode: true`, PhantomClean enters watch mode after the initial scan completes. It monitors the scraped folder for new files and processes them automatically.

A debounce buffer collects incoming files for `watch_debounce_seconds` of silence before flushing them as a batch — so a scraper dumping many folders at once is handled gracefully.

A periodic reconcile sweep also runs every `watch_debounce_seconds × 5` seconds to catch any files the watcher missed.

---

## Output Format

Each cleaned file is written to the output folder mirroring the input structure:

```
organized/
  site-name/
    page-title/
      organized.json
      organized.csv   ← if csv in export_format
      organized.txt   ← if txt in export_format
```

### JSON Output Schema

```json
{
  "url": "https://example.com/page",
  "title": "Page Title",
  "content": "Cleaned text content...",
  "word_count": 1842,
  "quality_score": 0.94,
  "language": "en",
  "cleaned_at": "2026-03-18T12:00:00Z",
  "ai_used": "groq/llama-3.3-70b-versatile",
  "layer_used": "layer1",
  "crawled_at": "2026-03-18T11:58:00Z",
  "links": ["https://example.com/other-page"],
  "images": ["https://example.com/image.jpg"],
  "emails": [],
  "phones": []
}
```

---

## Resuming

PhantomClean tracks state in a local SQLite database at `~/.phantomclean/state.db`.

- `phantomclean start` — always does a fresh scan, resets any interrupted state
- `phantomclean resume` — picks up exactly where it left off
- `phantomclean reset` — wipes all state so everything gets re-processed

---

## Use With PhantomCrawl

PhantomClean is designed as the companion cleaner to [PhantomCrawl](https://github.com/var-raphael/PhantomCrawl). Point `folder_to_clean` at PhantomCrawl's output folder and run both simultaneously — PhantomClean will watch for new folders as PhantomCrawl scrapes them.

```bash
# Terminal 1
phantomcrawl start

# Terminal 2
phantomclean start
```

PhantomClean prefers `cleaned.json` (AI pre-cleaned by PhantomCrawl) over `raw.json` when both exist in the same folder, so you never double-clean.

---

## License

BSL (Business Source License) — free for personal and non-commercial use. See [LICENSE](LICENSE).

---

## Author

Built by **Raphael Samuel**, 18, Lagos, Nigeria.

Self-taught. Started coding on a phone. No bootcamp, no degree, just code.

- Portfolio: [var-raphael.vercel.app](https://var-raphael.vercel.app)
- GitHub: [github.com/var-raphael](https://github.com/var-raphael)

> *The companion cleaner to PhantomCrawl. Together they scraped Cloudflare.*

---

*If PhantomClean helped you, star the repo ⭐*
