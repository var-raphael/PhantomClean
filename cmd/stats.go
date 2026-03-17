package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/var-raphael/PhantomClean/storage"
)

func humanTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show cleaning statistics and file records",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := storage.Init()
		if err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}
		defer db.Close()

		done, skipped, failed, pending, rulesOnly, err := db.GetStats()
		if err != nil {
			fmt.Println(red("Error: " + err.Error()))
			os.Exit(1)
		}

		fmt.Println(bold(cyan("PhantomClean Stats")))
		fmt.Println(dim("------------------"))

		if done == 0 && skipped == 0 && failed == 0 && pending == 0 {
			fmt.Println(yellow("No cleaning has been run yet. Run: phantomclean start"))
			return
		}

		fmt.Printf("Done          : %s\n", green(fmt.Sprintf("%d", done)))
		fmt.Printf("Pending       : %s\n", yellow(fmt.Sprintf("%d", pending)))

		if failed > 0 {
			fmt.Printf("Failed        : %s\n", red(fmt.Sprintf("%d", failed)))
		} else {
			fmt.Printf("Failed        : %s\n", green("0"))
		}

		if skipped > 0 {
			fmt.Printf("Skipped       : %s\n", dim(fmt.Sprintf("%d", skipped)))
		} else {
			fmt.Printf("Skipped       : %s\n", green("0"))
		}

		if rulesOnly > 0 {
			fmt.Printf("Rules only    : %s %s\n",
				yellow(fmt.Sprintf("%d", rulesOnly)),
				dim("(run phantomclean clean-ai to retry with AI)"),
			)
		} else {
			fmt.Printf("Rules only    : %s\n", green("0"))
		}

		// File records
		records, err := db.GetAllRecords()
		if err != nil || len(records) == 0 {
			return
		}

		// Done files
		fmt.Println("\n" + bold("Organized Files"))
		fmt.Println(dim("---------------"))
		for _, r := range records {
			if r.Status != "done" {
				continue
			}
			aiLabel := dim("[" + r.AIUsed + "]")
			timeLabel := ""
			if r.CleanedAt != nil {
				timeLabel = dim(humanTime(*r.CleanedAt))
			}
			fmt.Printf("  %s %s %s score:%s %s\n",
				green("✓"),
				r.FilePath,
				aiLabel,
				green(fmt.Sprintf("%.2f", r.QualityScore)),
				timeLabel,
			)
		}

		// Pending files
		hasPending := false
		for _, r := range records {
			if r.Status == "pending" || r.Status == "failed" {
				if !hasPending {
					fmt.Println("\n" + bold(yellow("Pending / Failed")))
					fmt.Println(dim("----------------"))
					hasPending = true
				}
				icon := yellow("⏳")
				if r.Status == "failed" {
					icon = red("✗")
				}
				reason := ""
				if r.SkipReason != "" {
					reason = dim(" — " + r.SkipReason)
				}
				fmt.Printf("  %s %s%s\n", icon, r.FilePath, reason)
			}
		}

		// Skipped files
		hasSkipped := false
		for _, r := range records {
			if r.Status == "skipped" {
				if !hasSkipped {
					fmt.Println("\n" + bold(dim("Skipped")))
					fmt.Println(dim("-------"))
					hasSkipped = true
				}
				fmt.Printf("  %s %s %s\n",
					dim("–"),
					r.FilePath,
					dim(r.SkipReason),
				)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
