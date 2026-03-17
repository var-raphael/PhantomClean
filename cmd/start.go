package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/var-raphael/PhantomClean/cleaner"
	"github.com/var-raphael/PhantomClean/config"
	"github.com/var-raphael/PhantomClean/storage"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start cleaning and watching the scraped folder",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, db := loadConfig()
		defer db.Close()

		c, err := cleaner.New(cfg, db)
		if err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}

		fmt.Println(bold(cyan("PhantomClean")) + dim(" starting..."))
		fmt.Println(dim("Folder  : ") + cfg.FolderToClean)
		fmt.Println(dim("Output  : ") + cfg.OutputFolder)
		fmt.Println(dim("AI      : ") + fmt.Sprintf("%v", cfg.AI.Enabled))
		fmt.Println(dim("Watch   : ") + fmt.Sprintf("%v", cfg.WatchMode))
		fmt.Println(dim("Batch   : ") + fmt.Sprintf("%d folders per batch", cfg.ConcurrentFolders))
		fmt.Println()

		if err := c.Start(); err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}
	},
}

var resumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume an interrupted cleaning run",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, db := loadConfig()
		defer db.Close()

		c, err := cleaner.New(cfg, db)
		if err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}

		fmt.Println(bold(cyan("PhantomClean")) + dim(" resuming..."))
		fmt.Println()

		if err := c.Resume(); err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}
	},
}

var cleanAICmd = &cobra.Command{
	Use:   "clean-ai",
	Short: "Retry AI cleaning on files that only got rule-based cleaning",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, db := loadConfig()
		defer db.Close()

		c, err := cleaner.New(cfg, db)
		if err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}

		fmt.Println(bold(cyan("PhantomClean")) + dim(" retrying AI on rules-only files..."))
		fmt.Println()

		if err := c.CleanAI(); err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}
	},
}

// loadConfig loads cleaner.json and inits DB
// shared by start, resume, clean-ai
func loadConfig() (*config.Config, *storage.DB) {
	// Load .env
	loadEnv(".env")

	cfg, err := config.Load("cleaner.json")
	if err != nil {
		fmt.Println(red("Error loading cleaner.json: " + err.Error()))
		fmt.Println(dim("Run: phantomclean init"))
		os.Exit(1)
	}

	db, err := storage.Init()
	if err != nil {
		fmt.Println(red("Error: " + err.Error()))
		os.Exit(1)
	}

	return cfg, db
}

// loadEnv reads .env file and sets environment variables
func loadEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			if key != "" && val != "" {
				os.Setenv(key, val)
			}
		}
	}
}

func init() {
	rootCmd.AddCommand(startCmd)
	rootCmd.AddCommand(resumeCmd)
	rootCmd.AddCommand(cleanAICmd)
}
