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

// parseRepo splits a GitHub repository spec in "owner/name" form into owner and name.
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

// runImport imports the given skillIDs into the given targets ("claude", "codex").
// skillIDs is always an explicit list of skill names decided by the caller, with no special
// handling such as "empty means all skills" (an empty list simply yields an empty result). The only
// way to select "all skills" is the interactive picker, which returns the user's choice as an
// explicit list of skill names. Runs using the existing Client for each target.
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

// availableClients returns only the clients whose CLI exists on PATH (`<name> --version`
// succeeds with exit code 0).
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

// targetsFor converts the command-line --target value (or the value chosen in interactive mode)
// into the targets slice passed to runImport. It assumes the caller has already validated it via
// validateTarget (no validation happens here).
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

// validateTarget validates the --target value (or the target chosen in interactive mode).
// Anything other than "claude"/"codex"/"both" is rejected as an error before any client is
// touched (this prevents, for example, a typo like "cluade" from silently importing into
// both).
func validateTarget(target string) error {
	switch target {
	case "claude", "codex", "both":
		return nil
	default:
		return fmt.Errorf("invalid --target %q (want claude|codex|both)", target)
	}
}

// filterClients narrows allClients down to only those included in targets.
func filterClients(allClients map[string]*pluginclient.Client, targets []string) map[string]*pluginclient.Client {
	filtered := make(map[string]*pluginclient.Client, len(targets))
	for _, target := range targets {
		if c, ok := allClients[target]; ok {
			filtered[target] = c
		}
	}
	return filtered
}

// ensureMarketplaces tries EnsureMarketplace for each of the clients and excludes any target that
// fails from the returned set (no install is attempted for that target).
// The reason for a failure is printed to stderr.
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

// runImportCmd is the body logic of the import command. It takes the runner/picker as arguments so
// that it can be tested without depending on the real CLI (exec), the real GitHub API, or a real terminal.
//
// repo is a skill repository in "owner/name" form. If marketplaceName is empty the repository name is used; if source is
// empty `https://github.com/<repo>` is used.
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
