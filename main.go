package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/buddyh/blast-radius/internal/engine"
	"github.com/buddyh/blast-radius/internal/lsp"
	"github.com/spf13/cobra"
)

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func main() {
	root := &cobra.Command{
		Use:           "br",
		Short:         "Map the blast radius of a code change before you make it",
		Version:       "0.1.0",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	var (
		flagDepth    int
		flagLang     string
		flagKind     string
		flagJSON     bool
		flagTimeout  time.Duration
		flagCoChange bool
		flagCoLimit  int
	)
	analyzeCmd := &cobra.Command{
		Use:   "analyze <target> [path]",
		Short: "Map every consumer of a symbol before changing it (breaking + transitive)",
		Long: "Resolve a target — a symbol name or file:line[:col] — via the language " +
			"server, then report every reference and the transitive callers a change would " +
			"ripple to, classified breaking / ripple / test, with a risk + migration hint.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := engine.Options{
				Target: args[0], Depth: flagDepth, Lang: flagLang, Kind: flagKind,
				CoChange: flagCoChange, CoChangeLimit: flagCoLimit,
			}
			if len(args) > 1 {
				o.Dir = args[1]
			}
			ctx, cancel := context.WithTimeout(context.Background(), flagTimeout)
			defer cancel()
			rep, err := engine.Analyze(ctx, o)
			if err != nil {
				return err
			}
			format := "table"
			if flagJSON || !isTTY() {
				format = "json"
			}
			rep.Render(os.Stdout, format)
			return nil
		},
	}
	analyzeCmd.Flags().IntVar(&flagDepth, "depth", 3, "transitive caller depth")
	analyzeCmd.Flags().StringVar(&flagLang, "lang", "", "force a language: go|typescript|python|rust|cpp")
	analyzeCmd.Flags().StringVar(&flagKind, "kind", "auto", "target kind: auto|symbol|endpoint|column|config|text")
	analyzeCmd.Flags().BoolVar(&flagJSON, "json", false, "output JSON (for tooling)")
	analyzeCmd.Flags().DurationVar(&flagTimeout, "timeout", 45*time.Second, "overall timeout")
	analyzeCmd.Flags().BoolVar(&flagCoChange, "cochange", true, "include git temporal-coupling (co-change) analysis")
	analyzeCmd.Flags().IntVar(&flagCoLimit, "cochange-limit", 8, "max co-changed files to show")

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
