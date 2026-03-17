package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/var-raphael/PhantomClean/storage"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Wipe state and start fresh",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := storage.Init()
		if err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}
		defer db.Close()

		if err := db.Reset(); err != nil {
			fmt.Println(red("Error resetting state: " + err.Error()))
			os.Exit(1)
		}

		fmt.Println(green("✓") + " State wiped. Ready for a fresh clean.")
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
}
