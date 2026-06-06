package main

import (
	"fmt"
	"os"

	"github.com/buddyh/blast-radius/internal/lsp"
	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:           "br",
		Short:         "Map the blast radius of a code change before you make it",
		Version:       "0.1.0",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	analyzeCmd := &cobra.Command{
		Use:   "analyze <target> [path]",
		Short: "Map every consumer of a symbol before changing it (breaking + transitive)",
		Long: "Resolve a target — a function, method, type, or symbol — via the language " +
			"server, then report every reference and the transitive callers that a change " +
			"would ripple to, classified breaking / ripple / test, with a suggested change order.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("the LSP engine lands in the next commit")
		},
	}

	doctorCmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check which language servers blast-radius can use",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("blast-radius — language servers")
			missing := 0
			for _, s := range lsp.Servers {
				if s.Installed() {
					fmt.Printf("  ✓ %-22s %s\n", s.Lang, s.Path())
				} else {
					missing++
					fmt.Printf("  ✗ %-22s not installed — %s\n", s.Lang, s.Install)
				}
			}
			if missing > 0 {
				fmt.Printf("\n%d server(s) missing. Install the ones for languages you use.\n", missing)
			}
			return nil
		},
	}

	root.AddCommand(analyzeCmd, doctorCmd)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
