package pluginclient

import "encoding/json"

type Tool interface {
	Name() string
	MarketplaceListArgs() []string
	MarketplaceAddArgs(source string) []string
	InstallArgs(pluginID string) []string
	ParseMarketplaceEntries(stdout string) ([]MarketplaceEntry, error)
	ParseInstallResult(res Result) (ok bool, message string, err error)
}

// MarketplaceEntry は`<tool> plugin marketplace list --json`の1件分を表す。
// Sourceは登録時に渡されたsource(URL/ローカルパス)にあたる値のベストエフォートな
// 復元で、ツール/種別によって異なるJSONフィールドから取り出す。復元できない場合は
// 空文字列になる(その場合、呼び出し側はsourceの一致比較をスキップする)。
type MarketplaceEntry struct {
	Name   string
	Source string
}

// --- Claude ---

type ClaudeTool struct{}

func (ClaudeTool) Name() string { return "claude" }

func (ClaudeTool) MarketplaceListArgs() []string {
	return []string{"plugin", "marketplace", "list", "--json"}
}

func (ClaudeTool) MarketplaceAddArgs(source string) []string {
	return []string{"plugin", "marketplace", "add", source}
}

func (ClaudeTool) InstallArgs(pluginID string) []string {
	return []string{"plugin", "install", pluginID, "--yes", "--json"}
}

func (ClaudeTool) ParseMarketplaceEntries(stdout string) ([]MarketplaceEntry, error) {
	var raw []struct {
		Name string `json:"name"`
		Repo string `json:"repo"`
		Path string `json:"path"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal([]byte(stdout), &raw); err != nil {
		return nil, err
	}
	var entries []MarketplaceEntry
	for _, r := range raw {
		source := r.Repo
		if source == "" {
			source = r.Path
		}
		if source == "" {
			source = r.URL
		}
		entries = append(entries, MarketplaceEntry{Name: r.Name, Source: source})
	}
	return entries, nil
}

func (ClaudeTool) ParseInstallResult(res Result) (bool, string, error) {
	var payload struct {
		Outcome string `json:"outcome"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(res.Stdout), &payload); err != nil {
		return false, "", err
	}
	return payload.Outcome == "ok", payload.Message, nil
}

// --- Codex ---

type CodexTool struct{}

func (CodexTool) Name() string { return "codex" }

func (CodexTool) MarketplaceListArgs() []string {
	return []string{"plugin", "marketplace", "list", "--json"}
}

func (CodexTool) MarketplaceAddArgs(source string) []string {
	return []string{"plugin", "marketplace", "add", source, "--json"}
}

func (CodexTool) InstallArgs(pluginID string) []string {
	return []string{"plugin", "add", pluginID, "--json"}
}

func (CodexTool) ParseMarketplaceEntries(stdout string) ([]MarketplaceEntry, error) {
	var payload struct {
		Marketplaces []struct {
			Name              string `json:"name"`
			MarketplaceSource struct {
				Source string `json:"source"`
			} `json:"marketplaceSource"`
		} `json:"marketplaces"`
	}
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return nil, err
	}
	var entries []MarketplaceEntry
	for _, m := range payload.Marketplaces {
		entries = append(entries, MarketplaceEntry{Name: m.Name, Source: m.MarketplaceSource.Source})
	}
	return entries, nil
}

func (CodexTool) ParseInstallResult(res Result) (bool, string, error) {
	if res.ExitCode != 0 {
		return false, res.Stderr, nil
	}
	return true, res.Stdout, nil
}
