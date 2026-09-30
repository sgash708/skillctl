package main

import (
	"os"

	"github.com/sgash708/skillctl/internal/pluginclient"
	"github.com/sgash708/skillctl/internal/ui"
	"github.com/spf13/cobra"
)

// newImportCmd はimportコマンドのcobraラッパー。ロジック本体はimport.goのrunImportCmdに
// あり、そちらはフェイクのRunner/Pickerで100%テストしている。ここでは具象型(実exec/
// 実端末に依存するExecRunner・HuhPicker)を生成してrunImportCmdへ注入するだけで、実CLI
// (claude/codex)・実GitHub API・実端末が無いと決定的にテストできない配線部分のため、
// 意図的にカバレッジ対象から除外している(internal/ui/picker.goと同じ考え方)。この
// ファイルをRunE本体だけの小さなファイルに分けているのも、除外範囲をこの配線部分だけに
// 限定し、runImportCmd以下のロジックはカバレッジ計測の対象に残すため。
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
