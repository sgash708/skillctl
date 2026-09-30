package main

import (
	"fmt"
	"path/filepath"

	"github.com/sgash708/skillctl/internal/manifest"
	"github.com/sgash708/skillctl/internal/skillsrepo"
	"github.com/spf13/cobra"
)

func runGenerate(root string, meta manifest.Meta, check bool) ([]string, error) {
	skills, err := skillsrepo.Scan(root)
	if err != nil {
		return nil, err
	}
	if err := skillsrepo.CheckAllowedContents(root, skills); err != nil {
		return nil, err
	}
	if check {
		return manifest.Check(root, meta, skills)
	}
	_, err = manifest.Write(root, meta, skills)
	return nil, err
}

// defaultMarketplaceName は--name省略時のmarketplace名として、対象ディレクトリ名を返す。
func defaultMarketplaceName(root string) string {
	abs, _ := filepath.Abs(root) // Absが失敗するのはカレントディレクトリが取得できない場合のみ
	return filepath.Base(abs)
}

func newGenerateCmd() *cobra.Command {
	var check bool
	var name, owner string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "SKILL.mdからplugin.json/marketplace.jsonを生成する",
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
			stale, err := runGenerate(root, manifest.Meta{Name: name, Owner: owner}, check)
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
	cmd.Flags().StringVar(&name, "name", "", "marketplace名(省略時は対象ディレクトリ名)")
	cmd.Flags().StringVar(&owner, "owner", "", "marketplace.jsonのowner名(必須)")
	cmd.Flags().BoolVar(&check, "check", false, "生成せず、生成物が最新かどうかだけ確認する")
	return cmd
}
