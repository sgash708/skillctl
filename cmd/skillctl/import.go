package main

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/sgash708/skillctl/internal/catalog"
	"github.com/sgash708/skillctl/internal/pluginclient"
	"github.com/sgash708/skillctl/internal/ui"
)

// parseRepo は"owner/name"形式のGitHubリポジトリ指定をownerとnameへ分割する。
func parseRepo(repo string) (owner, name string, err error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid --repo %q (want owner/name)", repo)
	}
	return parts[0], parts[1], nil
}

type importResult struct {
	Skill   string
	Target  string
	OK      bool
	Message string
}

// runImport は指定されたskillID群を、指定されたtargets("claude","codex")へimportする。
// skillIDsは常に呼び出し側が確定させた明示的なskill名のリストであり、「空なら全skill」の
// ような特別扱いはしない(空なら結果も単に空になる)。「全skill」を選ぶ手段は対話モードの
// pickerだけであり、pickerがユーザーの選択結果を明示的なskill名リストとして返す。
// 既存のClientをtargetごとに使い分けて実行する。
func runImport(ctx context.Context, clients map[string]*pluginclient.Client, marketplaceName string, skillIDs []string, targets []string) []importResult {
	var results []importResult
	for _, skill := range skillIDs {
		for _, target := range targets {
			client, ok := clients[target]
			if !ok {
				continue
			}
			pluginID := skill + "@" + marketplaceName
			ok2, msg, err := client.Install(ctx, pluginID)
			if err != nil {
				results = append(results, importResult{Skill: skill, Target: target, OK: false, Message: err.Error()})
				continue
			}
			results = append(results, importResult{Skill: skill, Target: target, OK: ok2, Message: msg})
		}
	}
	return results
}

// availableClients はPATH上に存在する(`<name> --version`が終了コード0で成功する)
// CLIのクライアントだけを返す。
func availableClients(ctx context.Context, runner pluginclient.Runner) map[string]*pluginclient.Client {
	clients := map[string]*pluginclient.Client{}
	if res, err := runner.Run(ctx, "claude", "--version"); err == nil && res.ExitCode == 0 {
		clients["claude"] = pluginclient.NewClient(pluginclient.ClaudeTool{}, runner)
	}
	if res, err := runner.Run(ctx, "codex", "--version"); err == nil && res.ExitCode == 0 {
		clients["codex"] = pluginclient.NewClient(pluginclient.CodexTool{}, runner)
	}
	return clients
}

// targetsFor はコマンドラインの --target 値(または対話モードで選ばれた値)を、
// runImportへ渡すtargetsのスライスへ変換する。呼び出し側でvalidateTargetにより
// 検証済みであることが前提(ここでは検証しない)。
func targetsFor(target string) []string {
	switch target {
	case "claude":
		return []string{"claude"}
	case "codex":
		return []string{"codex"}
	default:
		return []string{"claude", "codex"}
	}
}

// validateTarget は --target (または対話modeで選ばれたtarget)の値を検証する。
// "claude"/"codex"/"both"以外は、どのclientにも一切触れる前にエラーとして
// 弾く(例えばtypoで"cluade"のような値を渡した場合に、両方へ勝手にimportして
// しまうことを防ぐ)。
func validateTarget(target string) error {
	switch target {
	case "claude", "codex", "both":
		return nil
	default:
		return fmt.Errorf("invalid --target %q (want claude|codex|both)", target)
	}
}

// filterClients はallClientsを、targetsに含まれるものだけへ絞り込む。
func filterClients(allClients map[string]*pluginclient.Client, targets []string) map[string]*pluginclient.Client {
	filtered := make(map[string]*pluginclient.Client, len(targets))
	for _, target := range targets {
		if c, ok := allClients[target]; ok {
			filtered[target] = c
		}
	}
	return filtered
}

// ensureMarketplaces はclientsそれぞれについてEnsureMarketplaceを試み、失敗した
// targetは戻り値の集合から除外する(そのtargetへのinstallは一切試みない)。
// 失敗の理由はstderrへ表示する。
func ensureMarketplaces(ctx context.Context, clients map[string]*pluginclient.Client, marketplaceName, source string, stderr io.Writer) map[string]*pluginclient.Client {
	active := make(map[string]*pluginclient.Client, len(clients))
	for target, c := range clients {
		if err := c.EnsureMarketplace(ctx, marketplaceName, source); err != nil {
			fmt.Fprintf(stderr, "warning: failed to register the marketplace for %s; skipping this target: %v\n", target, err)
			continue
		}
		active[target] = c
	}
	return active
}

func printResults(w io.Writer, results []importResult) {
	for _, r := range results {
		status := "OK"
		if !r.OK {
			status = "NG"
		}
		fmt.Fprintf(w, "[%s] %s -> %s: %s\n", status, r.Skill, r.Target, r.Message)
	}
}

// runImportCmd はimportコマンドの本体ロジック。runner/pickerを引数として受け取ることで、
// 実CLI(exec)・実GitHub API・実端末に依存せずテストできるようにしている。
//
// repoは"owner/name"形式のskillリポジトリ。marketplaceNameが空ならリポジトリ名を、sourceが
// 空なら`https://github.com/<repo>`を使う。
func runImportCmd(ctx context.Context, runner pluginclient.Runner, picker ui.Picker, args []string, target string, yes bool, repo, marketplaceName, source string, stdout, stderr io.Writer) error {
	repoOwner, repoName, err := parseRepo(repo)
	if err != nil {
		return err
	}
	if marketplaceName == "" {
		marketplaceName = repoName
	}
	if source == "" {
		source = "https://github.com/" + repo
	}

	if yes && len(args) == 0 {
		return fmt.Errorf("skillctl import --yes requires at least one skill name")
	}

	interactive := len(args) == 0 && !yes
	if !interactive {
		if err := validateTarget(target); err != nil {
			return err
		}
	}

	allClients := availableClients(ctx, runner)
	if len(allClients) == 0 {
		return fmt.Errorf("neither the claude nor the codex CLI was found in PATH")
	}

	var skillIDs []string
	var targets []string
	if interactive {
		plugins, err := catalog.Fetch(ctx, runner, repoOwner, repoName)
		if err != nil {
			return fmt.Errorf("failed to fetch the skill list: %w", err)
		}
		items := make([]ui.Item, 0, len(plugins))
		for _, p := range plugins {
			items = append(items, ui.Item{ID: p.Name, Description: p.Description})
		}
		picked, pickedTarget, err := picker.Pick(items)
		if err != nil {
			return err
		}
		if err := validateTarget(pickedTarget); err != nil {
			return err
		}
		skillIDs = picked
		targets = targetsFor(pickedTarget)
	} else {
		skillIDs = args
		targets = targetsFor(target)
	}

	clients := filterClients(allClients, targets)
	clients = ensureMarketplaces(ctx, clients, marketplaceName, source, stderr)
	if len(clients) == 0 {
		return fmt.Errorf("no target could be prepared for install (all marketplace registrations failed or no matching tool was available)")
	}

	sort.Strings(skillIDs)
	results := runImport(ctx, clients, marketplaceName, skillIDs, targets)
	printResults(stdout, results)

	for _, r := range results {
		if !r.OK {
			return fmt.Errorf("some imports failed")
		}
	}
	return nil
}
