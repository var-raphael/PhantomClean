package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate cleaner.json, regex.txt, prompt.txt and .env template",
	Run: func(cmd *cobra.Command, args []string) {
		generated := false

		// cleaner.json
		if _, err := os.Stat("cleaner.json"); err == nil {
			fmt.Println(dim("cleaner.json already exists, skipping"))
		} else {
			err := os.WriteFile("cleaner.json", []byte(cleanerTemplate), 0644)
			if err != nil {
				fmt.Println(red("Error creating cleaner.json: " + err.Error()))
				return
			}
			fmt.Println(green("✓") + " cleaner.json created")
			generated = true
		}

		// regex.txt
		if _, err := os.Stat("regex.txt"); err == nil {
			fmt.Println(dim("regex.txt already exists, skipping"))
		} else {
			err := os.WriteFile("regex.txt", []byte(regexTemplate), 0644)
			if err != nil {
				fmt.Println(red("Error creating regex.txt: " + err.Error()))
				return
			}
			fmt.Println(green("✓") + " regex.txt created")
			generated = true
		}

		// prompt.txt
		if _, err := os.Stat("prompt.txt"); err == nil {
			fmt.Println(dim("prompt.txt already exists, skipping"))
		} else {
			err := os.WriteFile("prompt.txt", []byte(promptTemplate), 0644)
			if err != nil {
				fmt.Println(red("Error creating prompt.txt: " + err.Error()))
				return
			}
			fmt.Println(green("✓") + " prompt.txt created")
			generated = true
		}

		// .env
		if _, err := os.Stat(".env"); err == nil {
			fmt.Println(dim(".env already exists, skipping"))
		} else {
			err := os.WriteFile(".env", []byte(envTemplate), 0644)
			if err != nil {
				fmt.Println(red("Error creating .env: " + err.Error()))
				return
			}
			fmt.Println(green("✓") + " .env created")
			generated = true
		}

		if generated {
			fmt.Println()
			fmt.Println(bold("Next steps:"))
			fmt.Println("  1. Edit " + cyan("cleaner.json") + " — set folder_to_clean to your PhantomCrawl scraped folder")
			fmt.Println("  2. Edit " + cyan(".env") + " — add your AI API keys")
			fmt.Println("  3. Edit " + cyan("regex.txt") + " — add custom boilerplate patterns")
			fmt.Println("  4. Run   " + cyan("phantomclean start") + " — begin cleaning")
		} else {
			fmt.Println(yellow("All config files already exist. Nothing generated."))
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}

var cleanerTemplate = `{
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
  "log_file": "./phantomclean.log",
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
}`

var regexTemplate = `# PhantomClean regex patterns
# Each line is a pattern to strip from content
# Lines starting with # are comments and are ignored
# Supports full Go regex syntax

# Navigation
(click here|read more|learn more|see more)
(back to top|scroll to top|jump to content)

# Social and sharing
(share this|follow us|subscribe now|join our newsletter)
(like us on|follow us on|connect with us)

# Legal boilerplate
(privacy policy|terms of service|all rights reserved|cookie policy)
(©\s*\d{4}|copyright \d{4})

# Paywalls and signups
(sign up|log in|create account|already a member)
(subscribe to read|continue reading|unlock this article)

# Meta and timestamps
\d+ min read
(published|updated|posted) on

# Special symbols
[©®™►•□■→←↑↓]

# Empty lines
^\s*$

# Ads
(advertisement|sponsored content|paid partnership|promoted)

# Add your custom patterns below this line
`

var promptTemplate = `You are a data cleaning assistant.
You will receive scraped web content that has already been rule based cleaned.
Your job is to:
1. Remove any remaining boilerplate, navigation, or noise
2. Extract the core meaningful content only
3. Structure it into clean organized text
4. Return only the cleaned content, no commentary
5. Preserve all factual information accurately
6. Keep the language natural and readable
7. If the content has no meaningful information return: NO_CONTENT

Return only the cleaned text. Nothing else. No notes, no explanations.`

var envTemplate = `# PhantomClean API Keys
# Reference these in cleaner.json using $VAR_NAME

# Groq (free tier: 500k tokens/day per key)
# Get keys at: console.groq.com
GROQ_KEY_1=
GROQ_KEY_2=

# OpenAI (fallback)
# Get keys at: platform.openai.com
OPENAI_KEY_1=

# Anthropic (fallback)
# Get keys at: console.anthropic.com
ANTHROPIC_KEY_1=
`
