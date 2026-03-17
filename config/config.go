package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AIProvider struct {
	Provider string   `json:"provider"`
	Model    string   `json:"model"`
	Keys     []string `json:"keys"`
}

type AIConfig struct {
	Enabled         bool         `json:"enabled"`
	FallbackToRules bool         `json:"fallback_to_rules"`
	OnlyIfNoCleaned bool         `json:"only_if_no_cleaned"`
	PromptFile      string       `json:"prompt_file"`
	Rotate          string       `json:"rotate"`
	TimeoutSeconds  int          `json:"timeout_seconds"`
	MaxRetries      int          `json:"max_retries"`
	Providers       []AIProvider `json:"providers"`
}

type OutputConfig struct {
	ZipOnComplete bool   `json:"zip_on_complete"`
	ZipName       string `json:"zip_name"`
}

type Config struct {
	ConcurrentFolders    int          `json:"concurrent_folders"`
	MaxFiles             int          `json:"max_files"`
	FolderToClean        string       `json:"folder_to_clean"`
	OutputFileName       string       `json:"output_file_name"`
	ExportFormat         []string     `json:"export_format"`
	OmitFolders          []string     `json:"omit_folders"`
	WatchMode            bool         `json:"watch_mode"`
	WatchDebounceSeconds int          `json:"watch_debounce_seconds"` // seconds to wait for burst to settle before flushing (default 2)
	Overwrite            bool         `json:"overwrite"`
	MinWordCount         int          `json:"min_word_count"`
	BoilerplateThreshold int          `json:"boilerplate_threshold"`
	QualityScoreMinimum  float64      `json:"quality_score_minimum"`
	Language             string       `json:"language"`
	OutputFolder         string       `json:"output_folder"`
	LogFile              string       `json:"log_file"`
	Resume               bool         `json:"resume"`
	ContentType          string       `json:"content_type"`
	StripNavLinks        bool         `json:"strip_nav_links"`
	Output               OutputConfig `json:"output"`
	AI                   AIConfig     `json:"ai"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read %s: %w", path, err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("invalid cleaner.json: %w", err)
	}

	// Resolve $ENV_VAR references in all provider key slices
	for i := range cfg.AI.Providers {
		cfg.AI.Providers[i].Keys = resolveEnvVars(cfg.AI.Providers[i].Keys)
	}

	if err := validate(&cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// resolveEnvVars replaces $VAR_NAME with actual env values
func resolveEnvVars(keys []string) []string {
	var resolved []string
	for _, k := range keys {
		if strings.HasPrefix(k, "$") {
			val := os.Getenv(strings.TrimPrefix(k, "$"))
			if val != "" {
				resolved = append(resolved, val)
			}
		} else if k != "" {
			resolved = append(resolved, k)
		}
	}
	return resolved
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func validate(cfg *Config) error {
	if cfg.FolderToClean == "" {
		return fmt.Errorf("folder_to_clean is required in cleaner.json")
	}

	// Expand ~ paths
	cfg.FolderToClean = expandPath(cfg.FolderToClean)
	cfg.OutputFolder = expandPath(cfg.OutputFolder)
	cfg.LogFile = expandPath(cfg.LogFile)

	if cfg.ConcurrentFolders <= 0 {
		cfg.ConcurrentFolders = 3
	}
	if cfg.WatchDebounceSeconds <= 0 {
		cfg.WatchDebounceSeconds = 2
	}
	if cfg.MinWordCount <= 0 {
		cfg.MinWordCount = 10
	}
	if cfg.BoilerplateThreshold <= 0 {
		cfg.BoilerplateThreshold = 3
	}
	if cfg.QualityScoreMinimum <= 0 {
		cfg.QualityScoreMinimum = 0.6
	}
	if cfg.OutputFileName == "" {
		cfg.OutputFileName = "organized"
	}
	if cfg.Language == "" {
		cfg.Language = "en"
	}
	if len(cfg.ExportFormat) == 0 {
		cfg.ExportFormat = []string{"json"}
	}
	if cfg.ContentType == "" {
		cfg.ContentType = "text"
	}
	return nil
}
