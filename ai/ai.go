package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/var-raphael/PhantomClean/config"
)

// maxInputWords is the threshold above which we chunk the document.
// ~2000 words ≈ ~2700 tokens — safe for all supported providers.
const maxInputWords = 2000

// maxOutputTokens is the cap on tokens the AI returns per chunk.
// Set high enough that even a dense 2000-word chunk doesn't get truncated.
const maxOutputTokens = 4000

type Client struct {
	cfg      *config.AIConfig
	prompt   string
	keyIndex map[string]int // tracks current key index per provider
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Request struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	Messages  []Message `json:"messages"`
}

type Response struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func New(cfg *config.AIConfig) (*Client, error) {
	prompt := cfg.PromptFile
	if cfg.PromptFile != "" {
		data, err := os.ReadFile(cfg.PromptFile)
		if err == nil {
			prompt = string(data)
		}
	}
	return &Client{
		cfg:      cfg,
		prompt:   prompt,
		keyIndex: make(map[string]int),
	}, nil
}

// Clean runs the AI cascade with chunking for large documents.
// Splits content into chunks of maxInputWords words, cleans each chunk,
// then joins the results. Falls back to rules if all providers fail.
func (c *Client) Clean(content string) (string, string, error) {
	if !c.cfg.Enabled || len(c.cfg.Providers) == 0 {
		return "", "rules", nil
	}

	chunks := chunkText(content, maxInputWords)

	var cleanedChunks []string
	var usedProvider string

	for _, chunk := range chunks {
		cleaned, provider, err := c.cleanChunk(chunk)
		if err != nil {
			// If any chunk fails entirely, fall back to rules for whole doc
			if c.cfg.FallbackToRules {
				return "", "rules", nil
			}
			return "", "rules", err
		}
		if cleaned == "NO_CONTENT" || cleaned == "" {
			continue // skip empty chunks, keep going
		}
		cleanedChunks = append(cleanedChunks, cleaned)
		usedProvider = provider
	}

	if len(cleanedChunks) == 0 {
		return "", "rules", nil
	}

	return strings.Join(cleanedChunks, "\n\n"), usedProvider, nil
}

// cleanChunk sends a single chunk through the provider cascade
func (c *Client) cleanChunk(chunk string) (string, string, error) {
	maxRetries := c.cfg.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 1
	}

	for _, provider := range c.cfg.Providers {
		if len(provider.Keys) == 0 {
			continue
		}

		var result string
		var err error
		for attempt := 0; attempt < maxRetries; attempt++ {
			result, err = c.tryProvider(provider, chunk)
			if err == nil {
				break
			}
			if attempt < maxRetries-1 {
				time.Sleep(2 * time.Second)
			}
		}

		if err != nil {
			continue // try next provider
		}

		if result == "NO_CONTENT" {
			return "NO_CONTENT", "rules", nil
		}

		label := provider.Provider + "/" + provider.Model
		return result, label, nil
	}

	if c.cfg.FallbackToRules {
		return "", "rules", nil
	}

	return "", "rules", fmt.Errorf("all AI providers exhausted")
}

// chunkText splits text into chunks of at most maxWords words,
// splitting on paragraph boundaries where possible.
func chunkText(text string, maxWords int) []string {
	words := strings.Fields(text)
	if len(words) <= maxWords {
		return []string{text}
	}

	// Split into paragraphs first, then group into chunks
	paragraphs := strings.Split(text, "\n\n")
	var chunks []string
	var current []string
	currentWords := 0

	for _, para := range paragraphs {
		paraWords := len(strings.Fields(para))

		// If a single paragraph exceeds maxWords, hard-split it
		if paraWords > maxWords {
			// Flush current buffer first
			if len(current) > 0 {
				chunks = append(chunks, strings.Join(current, "\n\n"))
				current = nil
				currentWords = 0
			}
			// Hard-split the large paragraph by words
			paraWordList := strings.Fields(para)
			for i := 0; i < len(paraWordList); i += maxWords {
				end := i + maxWords
				if end > len(paraWordList) {
					end = len(paraWordList)
				}
				chunks = append(chunks, strings.Join(paraWordList[i:end], " "))
			}
			continue
		}

		// If adding this paragraph would exceed maxWords, flush and start new chunk
		if currentWords+paraWords > maxWords && len(current) > 0 {
			chunks = append(chunks, strings.Join(current, "\n\n"))
			current = nil
			currentWords = 0
		}

		current = append(current, para)
		currentWords += paraWords
	}

	// Flush remaining
	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, "\n\n"))
	}

	return chunks
}

func (c *Client) tryProvider(provider config.AIProvider, content string) (string, error) {
	key := c.pickKey(provider)
	if key == "" {
		return "", fmt.Errorf("no keys available for %s", provider.Provider)
	}

	var apiURL string
	var authHeader string

	switch provider.Provider {
	case "groq":
		apiURL = "https://api.groq.com/openai/v1/chat/completions"
		authHeader = "Bearer " + key
	case "openai":
		apiURL = "https://api.openai.com/v1/chat/completions"
		authHeader = "Bearer " + key
	case "anthropic":
		return c.callAnthropic(provider, key, content)
	default:
		return "", fmt.Errorf("unknown provider: %s", provider.Provider)
	}

	return c.callOpenAICompat(apiURL, authHeader, provider.Model, content)
}

func (c *Client) callOpenAICompat(apiURL, authHeader, model, content string) (string, error) {
	reqBody := Request{
		Model:     model,
		MaxTokens: maxOutputTokens,
		Messages: []Message{
			{Role: "system", Content: c.prompt},
			{Role: "user", Content: content},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: c.getTimeout()}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("rate limit hit")
	}
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("server error: %d", resp.StatusCode)
	}

	var result Response
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("api error: %s", result.Error.Message)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return strings.TrimSpace(result.Choices[0].Message.Content), nil
}

type AnthropicRequest struct {
	Model     string    `json:"model"`
	MaxTokens int       `json:"max_tokens"`
	System    string    `json:"system"`
	Messages  []Message `json:"messages"`
}

type AnthropicResponse struct {
	Content []struct {
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) callAnthropic(provider config.AIProvider, key, content string) (string, error) {
	reqBody := AnthropicRequest{
		Model:     provider.Model,
		MaxTokens: maxOutputTokens,
		System:    c.prompt,
		Messages: []Message{
			{Role: "user", Content: content},
		},
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(data))
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", key)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{Timeout: c.getTimeout()}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == 429 {
		return "", fmt.Errorf("rate limit hit")
	}
	if resp.StatusCode >= 500 {
		return "", fmt.Errorf("server error: %d", resp.StatusCode)
	}

	var result AnthropicResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", err
	}

	if result.Error != nil {
		return "", fmt.Errorf("api error: %s", result.Error.Message)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from Anthropic")
	}

	return strings.TrimSpace(result.Content[0].Text), nil
}

// getTimeout returns the configured timeout duration
func (c *Client) getTimeout() time.Duration {
	if c.cfg.TimeoutSeconds > 0 {
		return time.Duration(c.cfg.TimeoutSeconds) * time.Second
	}
	return 15 * time.Second
}

// pickKey selects a key based on rotation strategy
func (c *Client) pickKey(provider config.AIProvider) string {
	if len(provider.Keys) == 0 {
		return ""
	}

	if c.cfg.Rotate == "random" {
		return provider.Keys[rand.Intn(len(provider.Keys))]
	}

	// sequential
	idx := c.keyIndex[provider.Provider]
	if idx >= len(provider.Keys) {
		idx = 0 // wrap around instead of exhausting keys
	}
	key := provider.Keys[idx]
	c.keyIndex[provider.Provider] = idx + 1
	return key
}
