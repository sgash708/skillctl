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
		Short: "skillをClaude Code/Codexへimportする",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runImportCmd(cmd.Context(), pluginclient.ExecRunner{}, ui.HuhPicker{}, args, target, yes, repo, marketplace, source, cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	cmd.Flags().StringVar(&target, "target", "both", "import先: claude|codex|both")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "非対話モードで実行する(引数のskillを対象にする)")
	cmd.Flags().StringVar(&repo, "repo", os.Getenv("SKILLCTL_REPO"), "skillを管理しているGitHubリポジトリ(owner/name)。環境変数SKILLCTL_REPOでも指定できる")
	cmd.Flags().StringVar(&marketplace, "marketplace", "", "marketplace名(generateの--nameと揃える。省略時はリポジトリ名)")
	cmd.Flags().StringVar(&source, "source", "", "marketplaceのsource(省略時はhttps://github.com/<repo>。ローカル開発時はローカルディレクトリのパスに差し替えられる)")
	return cmd
}
