package main

import (
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "skillctl",
		Short: "GitHub上のskillリポジトリをClaude Code/Codexへimportし、marketplace/plugin manifestを生成するツール",
	}
	root.AddCommand(newGenerateCmd())
	root.AddCommand(newImportCmd())
	return root
}

func main() {
	if err := newRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
