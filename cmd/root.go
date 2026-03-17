package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "phantomclean",
	Short: "A powerful AI-assisted dataset cleaner and organizer",
	Long:  `PhantomClean - Cleans and organizes PhantomCrawl scraped data into structured datasets.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
