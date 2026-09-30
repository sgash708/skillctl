package pluginclient

import (
	"context"
	"fmt"
	"strings"
)

type Client struct {
	Tool   Tool
	Runner Runner
}

func NewClient(tool Tool, runner Runner) *Client {
	return &Client{Tool: tool, Runner: runner}
}

// normalizeMarketplaceSource は、末尾の"/"や".git"といった些細な表記差分を無視して
// sourceを比較できるようにする。
func normalizeMarketplaceSource(s string) string {
	s = strings.TrimSuffix(s, "/")
	s = strings.TrimSuffix(s, ".git")
	return s
}

func (c *Client) EnsureMarketplace(ctx context.Context, marketplaceName, source string) error {
	listRes, err := c.Runner.Run(ctx, c.Tool.Name(), c.Tool.MarketplaceListArgs()...)
	if err != nil {
		return fmt.Errorf("%s: marketplace list: %w", c.Tool.Name(), err)
	}
	entries, err := c.Tool.ParseMarketplaceEntries(listRes.Stdout)
	if err != nil {
		return fmt.Errorf("%s: parse marketplace list: %w", c.Tool.Name(), err)
	}
	for _, e := range entries {
		if e.Name != marketplaceName {
			continue
		}
		// e.Sourceが空(=このツール/エントリからは登録元を復元できなかった)場合は
		// 判断材料が無いので、これまで通り「登録済みなら何もしない」に倒す。
		if e.Source != "" && normalizeMarketplaceSource(e.Source) != normalizeMarketplaceSource(source) {
			return fmt.Errorf("%s: marketplace %q is already registered from a different source (%s); run `%s plugin marketplace remove %s` first if you want to switch sources", c.Tool.Name(), marketplaceName, e.Source, c.Tool.Name(), marketplaceName)
		}
		return nil
	}

	addRes, err := c.Runner.Run(ctx, c.Tool.Name(), c.Tool.MarketplaceAddArgs(source)...)
	if err != nil {
		return fmt.Errorf("%s: marketplace add: %w", c.Tool.Name(), err)
	}
	if addRes.ExitCode != 0 {
		return fmt.Errorf("%s: marketplace add failed (exit %d): %s", c.Tool.Name(), addRes.ExitCode, addRes.Stderr)
	}
	return nil
}

func (c *Client) Install(ctx context.Context, pluginID string) (bool, string, error) {
	res, err := c.Runner.Run(ctx, c.Tool.Name(), c.Tool.InstallArgs(pluginID)...)
	if err != nil {
		return false, "", fmt.Errorf("%s: install: %w", c.Tool.Name(), err)
	}
	return c.Tool.ParseInstallResult(res)
}
