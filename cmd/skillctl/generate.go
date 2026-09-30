package main

import (
	"fmt"
	"path/filepath"

	"github.com/sgash708/skillctl/internal/manifest"
	"github.com/sgash708/skillctl/internal/skillsrepo"
	"github.com/spf13/cobra"
)

// runGenerate はroot配下のskillからmanifestを生成(checkならstaleの確認だけ)する。
// strictなら、skillディレクトリに許可リスト外のファイル(hooks/・.mcp.json等)があるとエラーにする。
func runGenerate(root string, meta manifest.Meta, check, strict bool) ([]string, error) {
	skills, err := skillsrepo.Scan(root)
	if err != nil {
		return nil, err
	}
	if strict {
		if err := skillsrepo.CheckAllowedContents(root, skills); err != nil {
			return nil, err
		}
	}
	if check {
		return manifest.Check(root, meta, skills)
	}
	_, err = manifest.Write(root, meta, skills)
	return nil, err
}

// defaultMarketplaceName returns the root directory name, used when --name is omitted.
func defaultMarketplaceName(root string) string {
	abs, _ := filepath.Abs(root) // Abs fails only when the working directory cannot be determined
	return filepath.Base(abs)
}

func newGenerateCmd() *cobra.Command {
	var check, strict bool
	var name, owner string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate plugin.json/marketplace.json from SKILL.md files",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			if owner == "" {
				return fmt.Errorf("--owner is required")
			}
			if name == "" {
				name = defaultMarketplaceName(root)
			}
			stale, err := runGenerate(root, manifest.Meta{Name: name, Owner: owner}, check, strict)
			if err != nil {
				return err
			}
			if check && len(stale) > 0 {
				for _, f := range stale {
					fmt.Fprintln(cmd.ErrOrStderr(), "stale:", f)
				}
				return fmt.Errorf("generate --check: %d file(s) are stale, run `skillctl generate` to fix", len(stale))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "marketplace name (default: the root directory name)")
	cmd.Flags().StringVar(&owner, "owner", "", "owner name written to marketplace.json (required)")
	cmd.Flags().BoolVar(&strict, "strict", false, "reject skill directories that contain anything other than SKILL.md, README.md, .claude-plugin/, agents/ and assets/ (images only)")
	cmd.Flags().BoolVar(&check, "check", false, "do not write; only check that the generated files are up to date")
	return cmd
}
