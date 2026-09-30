package catalog

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/sgash708/skillctl/internal/manifest"
	"github.com/sgash708/skillctl/internal/pluginclient"
)

func Fetch(ctx context.Context, runner pluginclient.Runner, owner, repo string) ([]manifest.Plugin, error) {
	path := fmt.Sprintf("repos/%s/%s/contents/.claude-plugin/marketplace.json", owner, repo)
	res, err := runner.Run(ctx, "gh", "api", path)
	if err != nil {
		return nil, fmt.Errorf("catalog: gh api: %w", err)
	}
	if res.ExitCode != 0 {
		return nil, fmt.Errorf("catalog: gh api failed (exit %d): %s", res.ExitCode, res.Stderr)
	}

	var payload struct {
		Content string `json:"content"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		return nil, fmt.Errorf("catalog: parse gh api response: %w", err)
	}

	decoded, err := base64.StdEncoding.DecodeString(payload.Content)
	if err != nil {
		return nil, fmt.Errorf("catalog: decode content: %w", err)
	}

	var mp manifest.Marketplace
	if err := json.Unmarshal(decoded, &mp); err != nil {
		return nil, fmt.Errorf("catalog: parse marketplace.json: %w", err)
	}

	return mp.Plugins, nil
}
