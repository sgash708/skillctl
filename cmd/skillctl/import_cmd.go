package main

import (
	"os"

	"github.com/sgash708/skillctl/internal/pluginclient"
	"github.com/sgash708/skillctl/internal/ui"
	"github.com/spf13/cobra"
)

// newImportCmd is the cobra wrapper for the import command. The body logic lives in runImportCmd in import.go,
// which is tested to 100% with a fake Runner/Picker. This file only creates the concrete types (ExecRunner and HuhPicker, which depend on
// real exec / a real terminal) and injects them into runImportCmd. It is wiring that cannot be tested deterministically without the real CLIs
// (claude/codex), the real GitHub API, and a real terminal, so it is
// intentionally excluded from coverage (same idea as internal/ui/picker.go). The reason this
// file is split out as a small file containing only the RunE body is to limit the exclusion to just this wiring
// and keep the logic below runImportCmd within coverage measurement.
func newImportCmd() *cobra.Command {
	var target string
	var yes bool
	var repo, marketplace, source string

	cmd := &cobra.Command{
		Use:   "import [skill...]",
		Short: "Import skills into Claude Code / Codex",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportCmd(cmd.Context(), pluginclient.ExecRunner{}, ui.HuhPicker{}, args, target, yes, repo, marketplace, source, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&target, "target", "both", "where to import: claude|codex|both")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "non-interactive mode: import the skills given as arguments")
	cmd.Flags().StringVar(&repo, "repo", os.Getenv("SKILLCTL_REPO"), "GitHub repository that hosts the skills (owner/name); can also be set via SKILLCTL_REPO")
	cmd.Flags().StringVar(&marketplace, "marketplace", "", "marketplace name (must match `generate --name`; default: the repository name)")
	cmd.Flags().StringVar(&source, "source", "", "marketplace source (default: https://github.com/<repo>; a local directory path also works)")
	return cmd
}
