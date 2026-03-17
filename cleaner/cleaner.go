package cleaner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/var-raphael/PhantomClean/ai"
	"github.com/var-raphael/PhantomClean/config"
	"github.com/var-raphael/PhantomClean/output"
	"github.com/var-raphael/PhantomClean/storage"
)

type Cleaner struct {
	cfg             *config.Config
	db              *storage.DB
	aiClient        *ai.Client
	patterns        []*regexp.Regexp // pre-compiled at startup
	whitespaceRe    *regexp.Regexp
	backtickRe      *regexp.Regexp
	preRe           *regexp.Regexp
	codeRe          *regexp.Regexp
	mu              sync.Mutex
	doneCount       int
	boilerplateFreq map[string]int
	boilerplateMu   sync.Mutex
}

type RawPage struct {
	URL       string   `json:"url"`
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	Text      string   `json:"text"`
	HTML      string   `json:"html"`
	Raw       string   `json:"raw"`
	LayerUsed string   `json:"layer_used"`
	CrawledAt string   `json:"crawled_at"`
	Links     []string `json:"links"`
	Images    []string `json:"images"`
	Emails    []string `json:"emails"`
	Phones    []string `json:"phones"`
	Metadata  struct {
		Description string `json:"description"`
	} `json:"metadata"`
}

func New(cfg *config.Config, db *storage.DB) (*Cleaner, error) {
	rawPatterns, err := loadPatterns("regex.txt")
	if err != nil {
		rawPatterns = []string{}
	}

	// Pre-compile all patterns once at startup — not per file
	var compiled []*regexp.Regexp
	for _, p := range rawPatterns {
		re, err := regexp.Compile("(?i)" + p)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}

	aiClient, err := ai.New(&cfg.AI)
	if err != nil {
		return nil, fmt.Errorf("could not init AI client: %w", err)
	}

	return &Cleaner{
		cfg:             cfg,
		db:              db,
		aiClient:        aiClient,
		patterns:        compiled,
		whitespaceRe:    regexp.MustCompile(`\s+`),
		backtickRe:      regexp.MustCompile("(?s)```.*?```"),
		preRe:           regexp.MustCompile("(?si)<pre[^>]*>.*?</pre>"),
		codeRe:          regexp.MustCompile("(?si)<code[^>]*>.*?</code>"),
		boilerplateFreq: make(map[string]int),
	}, nil
}

// Start always does a fresh run:
//  1. Reset pending/failed DB entries so the batch scanner picks up everything
//  2. Run the full batched scan — blocks until ALL folders are processed
//  3. Only then hand off to the debounced watcher (if watch_mode: true)
func (c *Cleaner) Start() error {
	// Clear any leftover pending/failed from a previous interrupted run.
	// done/skipped are preserved so already-cleaned files are not re-processed.
	if err := c.db.ResetPending(); err != nil {
		return fmt.Errorf("could not reset pending state: %w", err)
	}

	fmt.Println("Scanning existing files...")
	if err := c.scanFolder(c.cfg.FolderToClean); err != nil {
		return err
	}
	fmt.Println(green("✓") + " Initial scan complete.")

	if !c.cfg.WatchMode {
		c.zipIfDone()
		return nil
	}

	// Full scan is done — now enter watch mode for new files only
	return c.watch()
}

func (c *Cleaner) Resume() error {
	pending, err := c.db.GetPending()
	if err != nil {
		return err
	}

	if len(pending) == 0 {
		fmt.Println("No pending files found. Scanning for new files...")
		return c.scanFolder(c.cfg.FolderToClean)
	}

	fmt.Printf("Resuming %d pending files...\n", len(pending))
	for _, path := range pending {
		c.processFile(path)
	}

	if c.cfg.WatchMode {
		return c.watch()
	}

	c.zipIfDone()
	return nil
}

// CleanAI retries AI on files that were only rule-cleaned
func (c *Cleaner) CleanAI() error {
	paths, err := c.db.GetRulesOnly()
	if err != nil {
		return err
	}

	if len(paths) == 0 {
		fmt.Println("No rules-only files found. All files already AI cleaned.")
		return nil
	}

	fmt.Printf("Retrying AI on %d rules-only files...\n", len(paths))
	for _, path := range paths {
		c.processFileAIOnly(path)
	}
	return nil
}

func (c *Cleaner) scanFolder(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return fmt.Errorf("could not read folder: %w", err)
	}

	// Collect all valid top-level site folders
	var folders []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		omitted := false
		for _, omit := range c.cfg.OmitFolders {
			if strings.EqualFold(entry.Name(), omit) {
				omitted = true
				break
			}
		}
		if !omitted {
			folders = append(folders, filepath.Join(root, entry.Name()))
		}
	}

	batchSize := c.cfg.ConcurrentFolders
	if batchSize <= 0 {
		batchSize = 3
	}

	total := len(folders)
	totalBatches := (total + batchSize - 1) / batchSize
	fmt.Printf("  %s Found %d folders — processing in batches of %d (%d batches total)\n",
		dim("→"), total, batchSize, totalBatches)

	// Process folders in strict sequential batches.
	// Each batch runs its folders concurrently, then wg.Wait() blocks
	// until every folder in the batch is done before the next batch starts.
	for i := 0; i < len(folders); i += batchSize {
		end := i + batchSize
		if end > len(folders) {
			end = len(folders)
		}
		batch := folders[i:end]
		batchNum := (i / batchSize) + 1

		fmt.Printf("  %s Batch %d/%d\n", dim("→"), batchNum, totalBatches)

		var wg sync.WaitGroup
		for _, folder := range batch {
			wg.Add(1)
			fmt.Printf("    %s %s\n", cyan("→"), filepath.Base(folder))
			go func(fp string) {
				defer wg.Done()
				c.processFolder(fp)
			}(folder)
		}
		wg.Wait() // ← blocks here until ALL folders in this batch are done
	}

	return nil
}

// processFolder walks a site folder at ANY depth and processes
// raw.json / cleaned.json files. cleaned.json takes priority over
// raw.json when both exist in the same directory.
func (c *Cleaner) processFolder(folderPath string) {
	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Skip directories — we only act on files
		if info.IsDir() {
			return nil
		}

		name := filepath.Base(path)

		// Only care about raw.json and cleaned.json
		if name != "raw.json" && name != "cleaned.json" {
			return nil
		}

		// If this is raw.json, skip it when cleaned.json exists in the same dir
		if name == "raw.json" {
			cleanedSibling := filepath.Join(filepath.Dir(path), "cleaned.json")
			if _, err := os.Stat(cleanedSibling); err == nil {
				return nil // cleaned.json takes priority
			}
		}

		// Resume / overwrite check
		if c.cfg.Resume && !c.cfg.Overwrite {
			if c.db.IsProcessed(path) {
				return nil
			}
		}

		c.processFile(path)
		return nil
	})
}

// watch monitors the scraped folder for new files arriving after the
// initial scan. It uses a debounce buffer to accumulate incoming Create
// events and process them in batches, so a scraper dumping many folders
// at once is handled gracefully rather than firing one goroutine per file.
func (c *Cleaner) watch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Watch all existing directories so we catch files dropped into them.
	// We do NOT process files here — the initial scan already handled them.
	filepath.Walk(c.cfg.FolderToClean, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() {
			watcher.Add(path)
		}
		return nil
	})

	debounceSeconds := c.cfg.WatchDebounceSeconds
	if debounceSeconds <= 0 {
		debounceSeconds = 2
	}
	debounce := time.Duration(debounceSeconds) * time.Second

	fmt.Printf(cyan("→")+" Watching for new files (debounce: %ds, batch: %d)...\n",
		debounceSeconds, c.cfg.ConcurrentFolders)

	// pending holds files that arrived during the current debounce window.
	// seenInWindow deduplicates within a single window.
	pending := make(map[string]struct{})
	var pendingMu sync.Mutex
	var debounceTimer *time.Timer

	// reconcile does a full folder walk and processes anything the watcher
	// missed — files added before watch started, missed Create events, etc.
	// It is safe to call concurrently; IsProcessed guards against duplicates.
	reconcile := func() {
		var missed []string
		filepath.Walk(c.cfg.FolderToClean, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			name := filepath.Base(path)
			if name != "raw.json" && name != "cleaned.json" {
				return nil
			}
			if name == "raw.json" {
				sibling := filepath.Join(filepath.Dir(path), "cleaned.json")
				if _, err := os.Stat(sibling); err == nil {
					return nil
				}
			}
			if !c.db.IsProcessed(path) {
				missed = append(missed, path)
			}
			return nil
		})

		if len(missed) == 0 {
			return
		}

		batchSize := c.cfg.ConcurrentFolders
		if batchSize <= 0 {
			batchSize = 3
		}

		fmt.Printf("  %s Reconcile: found %d unprocessed file(s)\n", yellow("→"), len(missed))

		for i := 0; i < len(missed); i += batchSize {
			end := i + batchSize
			if end > len(missed) {
				end = len(missed)
			}
			batch := missed[i:end]

			var wg sync.WaitGroup
			for _, f := range batch {
				wg.Add(1)
				go func(fp string) {
					defer wg.Done()
					c.processFile(fp)
				}(f)
			}
			wg.Wait()
		}
	}

	flushPending := func() {
		pendingMu.Lock()
		if len(pending) == 0 {
			pendingMu.Unlock()
			// Even if no debounce-buffered files, run reconcile to catch
			// anything the watcher missed entirely (e.g. scraper dumped files
			// faster than fsnotify could emit events)
			go reconcile()
			return
		}

		// Snapshot and clear the pending set
		files := make([]string, 0, len(pending))
		for f := range pending {
			files = append(files, f)
		}
		pending = make(map[string]struct{})
		pendingMu.Unlock()

		batchSize := c.cfg.ConcurrentFolders
		if batchSize <= 0 {
			batchSize = 3
		}

		fmt.Printf("  %s Flushing %d new file(s) in batches of %d\n",
			dim("→"), len(files), batchSize)

		for i := 0; i < len(files); i += batchSize {
			end := i + batchSize
			if end > len(files) {
				end = len(files)
			}
			batch := files[i:end]

			var wg sync.WaitGroup
			for _, f := range batch {
				wg.Add(1)
				go func(fp string) {
					defer wg.Done()
					c.processFile(fp)
				}(f)
			}
			wg.Wait()
		}

		// After flushing debounce buffer, reconcile to catch anything missed
		go reconcile()
	}

	// Periodic reconcile ticker — runs every reconcileInterval regardless of
	// watcher events. This is the final safety net: if the scraper stops and
	// the debounce never fires again, this ticker still catches leftover files.
	reconcileInterval := time.Duration(debounceSeconds*5) * time.Second
	reconcileTicker := time.NewTicker(reconcileInterval)
	defer reconcileTicker.Stop()

	fmt.Printf(cyan("→")+" Reconcile sweep every %v\n", reconcileInterval)

	for {
		select {
		case <-reconcileTicker.C:
			go reconcile()

		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			if event.Op&fsnotify.Create != 0 {
				name := filepath.Base(event.Name)

				// Watch newly created subdirectories immediately
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					watcher.Add(event.Name)
					continue
				}

				// Only buffer raw.json / cleaned.json
				if name != "raw.json" && name != "cleaned.json" {
					continue
				}

				// Skip if already processed by the initial scan
				if c.db.IsProcessed(event.Name) {
					continue
				}

				// If raw.json arrives but cleaned.json already exists alongside
				// it, skip — cleaned.json will be (or has been) processed instead
				if name == "raw.json" {
					sibling := filepath.Join(filepath.Dir(event.Name), "cleaned.json")
					if _, sibErr := os.Stat(sibling); sibErr == nil {
						continue
					}
				}

				pendingMu.Lock()
				pending[event.Name] = struct{}{}
				pendingMu.Unlock()

				// Reset the debounce timer on every new arrival.
				// This means we wait debounce duration of silence before flushing,
				// so a burst of files from the scraper is collected into one batch.
				if debounceTimer != nil {
					debounceTimer.Stop()
				}
				debounceTimer = time.AfterFunc(debounce, flushPending)
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Println("Watcher error:", err)
		}
	}
}

func (c *Cleaner) processFile(path string) {
	rel, err := filepath.Rel(c.cfg.FolderToClean, filepath.Dir(path))
	if err != nil {
		rel = filepath.Dir(path)
	}
	outFolder := filepath.Join(c.cfg.OutputFolder, rel)

	c.db.MarkPending(path)

	data, err := os.ReadFile(path)
	if err != nil {
		c.db.MarkFailed(path, "could not read file")
		return
	}

	var page RawPage
	if err := json.Unmarshal(data, &page); err != nil {
		c.db.MarkFailed(path, "invalid json")
		return
	}

	// Get text content — prefer content, fall back to text, then html
	text := page.Content
	if text == "" {
		text = page.Text
	}
	if text == "" {
		text = page.HTML
	}
	if text == "" {
		c.db.MarkSkipped(path, "no text content found")
		return
	}

	// Layer 1: regex stripping (uses pre-compiled patterns)
	text = c.Strip(text, c.cfg.ContentType)

	// Layer 2: boilerplate frequency detection
	text = c.StripBoilerplate(text)

	// Layer 3: quality scoring
	score := Score(text)
	wordCount := CountWords(text)

	if wordCount < c.cfg.MinWordCount {
		c.db.MarkSkipped(path, fmt.Sprintf("word count %d below minimum %d", wordCount, c.cfg.MinWordCount))
		return
	}
	if score < c.cfg.QualityScoreMinimum {
		c.db.MarkSkipped(path, fmt.Sprintf("quality score %.2f below minimum %.2f", score, c.cfg.QualityScoreMinimum))
		return
	}

	// Layer 4: AI cascade
	aiUsed := "rules"
	if c.cfg.AI.Enabled {
		skipAI := c.cfg.AI.OnlyIfNoCleaned && filepath.Base(path) == "cleaned.json"
		if !skipAI {
			text, aiUsed = c.AIClean(text)
			wordCount = CountWords(text)
			score = Score(text)
		}
	}

	// Language filter
	if c.cfg.Language != "all" {
		if !IsEnglish(text) {
			c.db.MarkSkipped(path, "non-english content")
			return
		}
	}

	organized := &output.OrganizedData{
		URL:          page.URL,
		Title:        cleanTitle(page.Title),
		Content:      text,
		WordCount:    wordCount,
		QualityScore: score,
		Language:     DetectLanguage(text),
		CleanedAt:    time.Now().Format(time.RFC3339),
		AIUsed:       aiUsed,
		LayerUsed:    page.LayerUsed,
		CrawledAt:    page.CrawledAt,
		Links:        cleanLinks(page.Links, c.cfg.StripNavLinks),
		Images:       cleanImages(page.Images),
		Emails:       dedup(page.Emails),
		Phones:       dedup(page.Phones),
	}

	if err := output.Write(outFolder, c.cfg.OutputFileName, c.cfg.ExportFormat, organized); err != nil {
		c.db.MarkFailed(path, "write failed: "+err.Error())
		return
	}

	c.db.MarkDone(path, score, wordCount, strings.Join(c.cfg.ExportFormat, ","), aiUsed)

	c.mu.Lock()
	c.doneCount++
	count := c.doneCount
	c.mu.Unlock()

	fmt.Printf("  %s [%d] %s (%s) score:%.2f words:%d\n",
		green("✓"), count, rel, aiUsed, score, wordCount)
}

func (c *Cleaner) processFileAIOnly(path string) {
	rel, _ := filepath.Rel(c.cfg.FolderToClean, filepath.Dir(path))
	outFolder := filepath.Join(c.cfg.OutputFolder, rel)
	organizedPath := filepath.Join(outFolder, c.cfg.OutputFileName+".json")

	data, err := os.ReadFile(organizedPath)
	if err != nil {
		fmt.Printf("  ✗ could not read organized file: %s\n", path)
		return
	}

	var organized output.OrganizedData
	if err := json.Unmarshal(data, &organized); err != nil {
		fmt.Printf("  ✗ invalid organized json: %s\n", path)
		return
	}

	cleaned, provider, err := c.aiClient.Clean(organized.Content)
	if err != nil || cleaned == "" {
		fmt.Printf("  %s AI failed for: %s\n", red("✗"), path)
		return
	}

	organized.Content = cleaned
	organized.AIUsed = provider
	organized.WordCount = CountWords(cleaned)
	organized.QualityScore = Score(cleaned)
	organized.CleanedAt = time.Now().Format(time.RFC3339)

	if err := output.Write(outFolder, c.cfg.OutputFileName, c.cfg.ExportFormat, &organized); err != nil {
		fmt.Printf("  ✗ write failed: %s\n", path)
		return
	}

	c.db.MarkDone(path, organized.QualityScore, organized.WordCount,
		strings.Join(c.cfg.ExportFormat, ","), provider)

	fmt.Printf("  ✓ AI cleaned: %s (%s)\n", rel, provider)
}

func (c *Cleaner) zipIfDone() {
	if !c.cfg.Output.ZipOnComplete {
		return
	}

	if c.doneCount == 0 {
		fmt.Println(yellow("No files cleaned, skipping zip."))
		return
	}

	if _, err := os.Stat(c.cfg.OutputFolder); os.IsNotExist(err) {
		fmt.Println(yellow("Output folder empty, skipping zip."))
		return
	}

	zipPath, err := output.Zip(c.cfg.OutputFolder, c.cfg.Output.ZipName, c.doneCount)
	if err != nil {
		fmt.Println(red("Zip failed: " + err.Error()))
		return
	}
	fmt.Println("📦 Dataset zipped:", zipPath)
}
