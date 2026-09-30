package main

import (
	"os"

	"github.com/spf13/cobra"
)

// version is set at build time by goreleaser (-ldflags "-X main.version=...").
var version = "dev"

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Version:      version,
		SilenceUsage: true,
		Use:          "skillctl",
		Short:        "Import skills from a GitHub repository into Claude Code / Codex, and generate marketplace/plugin manifests",
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
