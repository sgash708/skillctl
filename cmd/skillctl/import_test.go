package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sgash708/skillctl/internal/manifest"
	"github.com/sgash708/skillctl/internal/pluginclient"
	"github.com/sgash708/skillctl/internal/ui"
)

// --- runImport ---

type stubTool struct {
	name string
}

func (s stubTool) Name() string                     { return s.name }
func (stubTool) MarketplaceListArgs() []string      { return nil }
func (stubTool) MarketplaceAddArgs(string) []string { return nil }
func (stubTool) InstallArgs(id string) []string     { return []string{id} }
func (stubTool) ParseMarketplaceEntries(string) ([]pluginclient.MarketplaceEntry, error) {
	return []pluginclient.MarketplaceEntry{{Name: "skills"}}, nil
}
func (stubTool) ParseInstallResult(res pluginclient.Result) (bool, string, error) {
	return res.ExitCode == 0, res.Stdout, nil
}

type stubRunner struct {
	fail map[string]bool // pluginID -> whether to make it fail
}

func (r stubRunner) Run(ctx context.Context, name string, args ...string) (pluginclient.Result, error) {
	pluginID := args[0]
	if r.fail[pluginID] {
		return pluginclient.Result{ExitCode: 1, Stderr: "boom"}, nil
	}
	return pluginclient.Result{ExitCode: 0, Stdout: "ok:" + pluginID}, nil
}

// erroringRunner is a stub whose Run itself returns an error (simulating a CLI launch failure, etc.).
type erroringRunner struct{}

func (erroringRunner) Run(ctx context.Context, name string, args ...string) (pluginclient.Result, error) {
	return pluginclient.Result{}, errors.New("exec failed")
}

func TestRunImport(t *testing.T) {
	tests := []struct {
		name     string
		clients  map[string]*pluginclient.Client
		skillIDs []string
		targets  []string
		want     map[string]bool // key "skill/target" -> OK
	}{
		{
			name: "mixed success and failure across two targets",
			clients: map[string]*pluginclient.Client{
				"claude": pluginclient.NewClient(stubTool{name: "claude"}, stubRunner{fail: map[string]bool{"broken-skill@skills": true}}),
				"codex":  pluginclient.NewClient(stubTool{name: "codex"}, stubRunner{}),
			},
			skillIDs: []string{"example-skill", "broken-skill"},
			targets:  []string{"claude", "codex"},
			want: map[string]bool{
				"example-skill/claude": true,
				"example-skill/codex":  true,
				"broken-skill/claude":  false,
				"broken-skill/codex":   true,
			},
		},
		{
			name: "unknown target is skipped when its client is missing",
			clients: map[string]*pluginclient.Client{
				"claude": pluginclient.NewClient(stubTool{name: "claude"}, stubRunner{}),
			},
			skillIDs: []string{"example-skill"},
			targets:  []string{"claude", "codex"},
			want: map[string]bool{
				"example-skill/claude": true,
			},
		},
		{
			name: "runner error itself is captured as a failed result",
			clients: map[string]*pluginclient.Client{
				"claude": pluginclient.NewClient(stubTool{name: "claude"}, erroringRunner{}),
			},
			skillIDs: []string{"example-skill"},
			targets:  []string{"claude"},
			want: map[string]bool{
				"example-skill/claude": false,
			},
		},
		{
			name: "no skills yields no results even with clients and targets",
			clients: map[string]*pluginclient.Client{
				"claude": pluginclient.NewClient(stubTool{name: "claude"}, stubRunner{}),
			},
			skillIDs: nil,
			targets:  []string{"claude"},
			want:     map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := runImport(context.Background(), tt.clients, "skills", tt.skillIDs, tt.targets)
			if len(results) != len(tt.want) {
				t.Fatalf("len(results) = %d, want %d (got=%+v)", len(results), len(tt.want), results)
			}
			for _, r := range results {
				key := r.Skill + "/" + r.Target
				wantOK, ok := tt.want[key]
				if !ok {
					t.Fatalf("unexpected result key %q", key)
				}
				if r.OK != wantOK {
					t.Errorf("result[%s].OK = %v, want %v", key, r.OK, wantOK)
				}
			}
		})
	}
}

// --- availableClients ---

// versionRunner simulates the result of `--version` per executable name (or a failure to launch at all).
type versionRunner struct {
	results map[string]pluginclient.Result
	errFor  map[string]bool
}

func (r versionRunner) Run(ctx context.Context, name string, args ...string) (pluginclient.Result, error) {
	if r.errFor[name] {
		return pluginclient.Result{}, errors.New(name + ": not found")
	}
	if res, ok := r.results[name]; ok {
		return res, nil
	}
	return pluginclient.Result{ExitCode: 0}, nil
}

func TestAvailableClients(t *testing.T) {
	tests := []struct {
		name       string
		runner     versionRunner
		wantClaude bool
		wantCodex  bool
	}{
		{
			name: "both claude and codex are on PATH and exit 0",
			runner: versionRunner{results: map[string]pluginclient.Result{
				"claude": {ExitCode: 0},
				"codex":  {ExitCode: 0},
			}},
			wantClaude: true,
			wantCodex:  true,
		},
		{
			name: "only claude is on PATH",
			runner: versionRunner{
				results: map[string]pluginclient.Result{"claude": {ExitCode: 0}},
				errFor:  map[string]bool{"codex": true},
			},
			wantClaude: true,
			wantCodex:  false,
		},
		{
			name: "only codex is on PATH",
			runner: versionRunner{
				results: map[string]pluginclient.Result{"codex": {ExitCode: 0}},
				errFor:  map[string]bool{"claude": true},
			},
			wantClaude: false,
			wantCodex:  true,
		},
		{
			name: "neither is on PATH (Run itself errors)",
			runner: versionRunner{
				errFor: map[string]bool{"claude": true, "codex": true},
			},
			wantClaude: false,
			wantCodex:  false,
		},
		{
			name: "claude binary exists but exits non-zero on --version",
			runner: versionRunner{results: map[string]pluginclient.Result{
				"claude": {ExitCode: 1},
				"codex":  {ExitCode: 0},
			}},
			wantClaude: false,
			wantCodex:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clients := availableClients(context.Background(), tt.runner)

			if _, ok := clients["claude"]; ok != tt.wantClaude {
				t.Errorf("clients[\"claude\"] present = %v, want %v", ok, tt.wantClaude)
			}
			if _, ok := clients["codex"]; ok != tt.wantCodex {
				t.Errorf("clients[\"codex\"] present = %v, want %v", ok, tt.wantCodex)
			}
			wantLen := 0
			if tt.wantClaude {
				wantLen++
			}
			if tt.wantCodex {
				wantLen++
			}
			if len(clients) != wantLen {
				t.Errorf("len(clients) = %d, want %d", len(clients), wantLen)
			}
		})
	}
}

// --- targetsFor ---

func TestTargetsFor(t *testing.T) {
	tests := []struct {
		name   string
		target string
		want   []string
	}{
		{name: "claude only", target: "claude", want: []string{"claude"}},
		{name: "codex only", target: "codex", want: []string{"codex"}},
		{name: "both (explicit)", target: "both", want: []string{"claude", "codex"}},
		{name: "unrecognized value defaults to both", target: "", want: []string{"claude", "codex"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := targetsFor(tt.target)
			if len(got) != len(tt.want) {
				t.Fatalf("targetsFor(%q) = %v, want %v", tt.target, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("targetsFor(%q)[%d] = %q, want %q", tt.target, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- printResults ---

func TestPrintResults(t *testing.T) {
	tests := []struct {
		name    string
		results []importResult
		want    string
	}{
		{
			name:    "no results prints nothing",
			results: nil,
			want:    "",
		},
		{
			name: "successful result is marked OK",
			results: []importResult{
				{Skill: "example-skill", Target: "claude", OK: true, Message: "installed"},
			},
			want: "[OK] example-skill -> claude: installed\n",
		},
		{
			name: "failed result is marked NG",
			results: []importResult{
				{Skill: "broken-skill", Target: "codex", OK: false, Message: "boom"},
			},
			want: "[NG] broken-skill -> codex: boom\n",
		},
		{
			name: "multiple results are printed in order",
			results: []importResult{
				{Skill: "example-skill", Target: "claude", OK: true, Message: "installed"},
				{Skill: "broken-skill", Target: "codex", OK: false, Message: "boom"},
			},
			want: "[OK] example-skill -> claude: installed\n[NG] broken-skill -> codex: boom\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			printResults(&buf, tt.results)
			if got := buf.String(); got != tt.want {
				t.Errorf("printResults() = %q, want %q", got, tt.want)
			}
		})
	}
}

// --- runImportCmd ---

// importCmdFakeRunner is a fake that returns a result/error per combination of exec name + arguments.
// (Same idea as fakeRunner in internal/pluginclient/client_test.go.) It records all calls in
// calls so tests can verify that "a specific target was never touched".
type importCmdFakeRunner struct {
	results map[string]pluginclient.Result
	errFor  map[string]error
	calls   []string
}

func cmdKey(name string, args []string) string {
	return name + " " + strings.Join(args, " ")
}

func (f *importCmdFakeRunner) Run(ctx context.Context, name string, args ...string) (pluginclient.Result, error) {
	key := cmdKey(name, args)
	f.calls = append(f.calls, key)
	if err, ok := f.errFor[key]; ok {
		return pluginclient.Result{}, err
	}
	if res, ok := f.results[key]; ok {
		return res, nil
	}
	return pluginclient.Result{ExitCode: 0}, nil
}

// fakePicker is a deterministic test double for ui.Picker.
type fakePicker struct {
	ids    []string
	target string
	err    error
}

func (p fakePicker) Pick(items []ui.Item) ([]string, string, error) {
	return p.ids, p.target, p.err
}

func TestRunImportCmd(t *testing.T) {
	claudeTool := pluginclient.ClaudeTool{}
	codexTool := pluginclient.CodexTool{}
	ghAPIKey := cmdKey("gh", []string{"api", "repos/example-org/skills/contents/.claude-plugin/marketplace.json"})

	tests := []struct {
		name          string
		runner        importCmdFakeRunner
		picker        fakePicker
		args          []string
		target        string
		yes           bool
		wantErrSubstr string
		wantStdoutHas []string
		wantStdoutIs  *string
		wantStderrHas []string
		wantStderrIs  *string
		// wantNoCallsContaining: after the run, verifies that no call in runner.calls contains this string
		// (e.g. specify "codex" and verify that codex was never
		// touched).
		wantNoCallsContaining string
		// wantNoCalls: verifies that runner.calls is completely empty (= no client was
		// touched at all).
		wantNoCalls bool
	}{
		{
			name: "no clients available on PATH returns an error and no output",
			runner: importCmdFakeRunner{
				errFor: map[string]error{
					cmdKey("claude", []string{"--version"}): errors.New("not found"),
					cmdKey("codex", []string{"--version"}):  errors.New("not found"),
				},
			},
			args:          []string{"example-skill"},
			target:        "claude",
			yes:           true,
			wantErrSubstr: "was found in PATH",
			wantStdoutIs:  strPtr(""),
		},
		{
			name: "marketplace registration failure for the only requested target returns an error (total failure)",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):                          {ExitCode: 0},
					cmdKey("claude", claudeTool.InstallArgs("example-skill@skills")): {ExitCode: 0, Stdout: `{"outcome":"ok","message":"ok"}`},
				},
				errFor: map[string]error{
					cmdKey("codex", []string{"--version"}):             errors.New("not found"),
					cmdKey("claude", claudeTool.MarketplaceListArgs()): errors.New("list failed"),
				},
			},
			args:                  []string{"example-skill"},
			target:                "claude",
			yes:                   true,
			wantErrSubstr:         "no target could be prepared for install",
			wantStdoutIs:          strPtr(""),
			wantStderrHas:         []string{"warning:"},
			wantNoCallsContaining: "install",
		},
		{
			name: "--target codex with only claude available on PATH returns an error, not a silent no-op",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}): {ExitCode: 0},
				},
				errFor: map[string]error{
					cmdKey("codex", []string{"--version"}): errors.New("not found"),
				},
			},
			args:                  []string{"example-skill"},
			target:                "codex",
			yes:                   true,
			wantErrSubstr:         "no target could be prepared for install",
			wantStdoutIs:          strPtr(""),
			wantStderrIs:          strPtr(""),
			wantNoCallsContaining: "codex plugin",
		},
		{
			name: "--yes with zero skill arguments is a validation error, not a silent no-op",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}): {ExitCode: 0},
				},
			},
			args:          nil,
			target:        "claude",
			yes:           true,
			wantErrSubstr: "skillctl import --yes requires at least one skill name",
			wantStdoutIs:  strPtr(""),
			wantNoCalls:   true,
		},
		{
			name: "invalid --target value is rejected before touching any client",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}): {ExitCode: 0},
					cmdKey("codex", []string{"--version"}):  {ExitCode: 0},
				},
			},
			args:          []string{"example-skill"},
			target:        "cluade", // typo
			yes:           true,
			wantErrSubstr: `invalid --target "cluade"`,
			wantStdoutIs:  strPtr(""),
			wantNoCalls:   true,
		},
		{
			name: "--target claude with both claude and codex on PATH only touches claude",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):                          {ExitCode: 0},
					cmdKey("codex", []string{"--version"}):                           {ExitCode: 0},
					cmdKey("claude", claudeTool.MarketplaceListArgs()):               {ExitCode: 0, Stdout: `[{"name":"skills"}]`},
					cmdKey("claude", claudeTool.InstallArgs("example-skill@skills")): {ExitCode: 0, Stdout: `{"outcome":"ok","message":"ok"}`},
				},
			},
			args:                  []string{"example-skill"},
			target:                "claude",
			yes:                   true,
			wantStdoutHas:         []string{"[OK] example-skill -> claude: ok"},
			wantStderrIs:          strPtr(""),
			wantNoCallsContaining: "codex plugin",
		},
		{
			name: "--target both: one target's EnsureMarketplace failure doesn't affect the other target",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):                        {ExitCode: 0},
					cmdKey("codex", []string{"--version"}):                         {ExitCode: 0},
					cmdKey("codex", codexTool.MarketplaceListArgs()):               {ExitCode: 0, Stdout: `{"marketplaces":[{"name":"skills"}]}`},
					cmdKey("codex", codexTool.InstallArgs("example-skill@skills")): {ExitCode: 0, Stdout: "codex-ok"},
				},
				errFor: map[string]error{
					cmdKey("claude", claudeTool.MarketplaceListArgs()): errors.New("list failed"),
				},
			},
			args:          []string{"example-skill"},
			target:        "both",
			yes:           true,
			wantStdoutHas: []string{"[OK] example-skill -> codex: codex-ok"},
			wantStderrHas: []string{"warning:"},
		},
		{
			name: "non-interactive multi-target success with no marketplace warnings",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):                          {ExitCode: 0},
					cmdKey("codex", []string{"--version"}):                           {ExitCode: 0},
					cmdKey("claude", claudeTool.MarketplaceListArgs()):               {ExitCode: 0, Stdout: `[{"name":"skills"}]`},
					cmdKey("codex", codexTool.MarketplaceListArgs()):                 {ExitCode: 0, Stdout: `{"marketplaces":[{"name":"skills"}]}`},
					cmdKey("claude", claudeTool.InstallArgs("example-skill@skills")): {ExitCode: 0, Stdout: `{"outcome":"ok","message":"claude-ok"}`},
					cmdKey("codex", codexTool.InstallArgs("example-skill@skills")):   {ExitCode: 0, Stdout: "codex-ok"},
				},
			},
			args:   []string{"example-skill"},
			target: "both",
			yes:    true,
			wantStdoutHas: []string{
				"[OK] example-skill -> claude: claude-ok",
				"[OK] example-skill -> codex: codex-ok",
			},
			wantStderrIs: strPtr(""),
		},
		{
			name: "interactive mode: picker success drives skill/target selection",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):            {ExitCode: 0},
					cmdKey("claude", claudeTool.MarketplaceListArgs()): {ExitCode: 0, Stdout: `[{"name":"skills"}]`},
					ghAPIKey: mustGhAPIResult([]manifest.Plugin{{Name: "example-skill", Description: "desc"}}),
					cmdKey("claude", claudeTool.InstallArgs("example-skill@skills")): {ExitCode: 0, Stdout: `{"outcome":"ok","message":"picked-ok"}`},
				},
				errFor: map[string]error{
					cmdKey("codex", []string{"--version"}): errors.New("not found"),
				},
			},
			picker:        fakePicker{ids: []string{"example-skill"}, target: "claude"},
			args:          nil,
			yes:           false,
			wantStdoutHas: []string{"[OK] example-skill -> claude: picked-ok"},
		},
		{
			name: "interactive mode: catalog fetch failure propagates as an error",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):            {ExitCode: 0},
					cmdKey("claude", claudeTool.MarketplaceListArgs()): {ExitCode: 0, Stdout: `[{"name":"skills"}]`},
				},
				errFor: map[string]error{
					cmdKey("codex", []string{"--version"}): errors.New("not found"),
					ghAPIKey:                               errors.New("network down"),
				},
			},
			args:          nil,
			yes:           false,
			wantErrSubstr: "failed to fetch the skill list",
		},
		{
			name: "interactive mode: picker failure propagates as an error",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):            {ExitCode: 0},
					cmdKey("claude", claudeTool.MarketplaceListArgs()): {ExitCode: 0, Stdout: `[{"name":"skills"}]`},
					ghAPIKey: mustGhAPIResult([]manifest.Plugin{{Name: "example-skill", Description: "desc"}}),
				},
				errFor: map[string]error{
					cmdKey("codex", []string{"--version"}): errors.New("not found"),
				},
			},
			picker:        fakePicker{err: errors.New("user cancelled")},
			args:          nil,
			yes:           false,
			wantErrSubstr: "user cancelled",
		},
		{
			name: "interactive mode: picker returns an invalid target",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):            {ExitCode: 0},
					cmdKey("claude", claudeTool.MarketplaceListArgs()): {ExitCode: 0, Stdout: `[{"name":"skills"}]`},
					ghAPIKey: mustGhAPIResult([]manifest.Plugin{{Name: "example-skill", Description: "desc"}}),
				},
				errFor: map[string]error{
					cmdKey("codex", []string{"--version"}): errors.New("not found"),
				},
			},
			picker:        fakePicker{ids: []string{"example-skill"}, target: "not-a-real-target"},
			args:          nil,
			yes:           false,
			wantErrSubstr: `invalid --target "not-a-real-target"`,
			wantStdoutIs:  strPtr(""),
		},
		{
			name: "partial failure across skills aggregates into a final error",
			runner: importCmdFakeRunner{
				results: map[string]pluginclient.Result{
					cmdKey("claude", []string{"--version"}):                          {ExitCode: 0},
					cmdKey("claude", claudeTool.MarketplaceListArgs()):               {ExitCode: 0, Stdout: `[{"name":"skills"}]`},
					cmdKey("claude", claudeTool.InstallArgs("example-skill@skills")): {ExitCode: 0, Stdout: `{"outcome":"ok","message":"ok"}`},
					cmdKey("claude", claudeTool.InstallArgs("broken-skill@skills")):  {ExitCode: 1, Stdout: `{"outcome":"error","message":"failed"}`},
				},
				errFor: map[string]error{
					cmdKey("codex", []string{"--version"}): errors.New("not found"),
				},
			},
			args:          []string{"example-skill", "broken-skill"},
			target:        "claude",
			yes:           true,
			wantErrSubstr: "some imports failed",
			wantStdoutHas: []string{
				"[OK] example-skill -> claude: ok",
				"[NG] broken-skill -> claude: failed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := runImportCmd(context.Background(), &tt.runner, tt.picker, tt.args, tt.target, tt.yes, "example-org/skills", "", "", &stdout, &stderr)

			if tt.wantNoCalls && len(tt.runner.calls) != 0 {
				t.Errorf("expected no runner calls at all, got %v", tt.runner.calls)
			}
			if tt.wantNoCallsContaining != "" {
				for _, c := range tt.runner.calls {
					if strings.Contains(c, tt.wantNoCallsContaining) {
						t.Errorf("runner.calls unexpectedly contains %q: %v", tt.wantNoCallsContaining, tt.runner.calls)
						break
					}
				}
			}

			if tt.wantErrSubstr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSubstr) {
					t.Fatalf("err = %v, want containing %q", err, tt.wantErrSubstr)
				}
			}

			gotStdout := stdout.String()
			for _, want := range tt.wantStdoutHas {
				if !strings.Contains(gotStdout, want) {
					t.Errorf("stdout = %q, want containing %q", gotStdout, want)
				}
			}
			if tt.wantStdoutIs != nil && gotStdout != *tt.wantStdoutIs {
				t.Errorf("stdout = %q, want %q", gotStdout, *tt.wantStdoutIs)
			}

			gotStderr := stderr.String()
			for _, want := range tt.wantStderrHas {
				if !strings.Contains(gotStderr, want) {
					t.Errorf("stderr = %q, want containing %q", gotStderr, want)
				}
			}
			if tt.wantStderrIs != nil && gotStderr != *tt.wantStderrIs {
				t.Errorf("stderr = %q, want %q", gotStderr, *tt.wantStderrIs)
			}
		})
	}
}

func strPtr(s string) *string { return &s }

// mustGhAPIResult is a helper to build, as an in-table literal, the `gh api` response that catalog.Fetch reads (the base64-encoded
// content of marketplace.json). The input to json.Marshal is a fixed struct
// so it cannot fail, and a panic is enough.
func mustGhAPIResult(plugins []manifest.Plugin) pluginclient.Result {
	mp := manifest.Marketplace{Plugins: plugins}
	body, err := json.Marshal(mp)
	if err != nil {
		panic(err)
	}
	payload := struct {
		Content string `json:"content"`
	}{Content: base64.StdEncoding.EncodeToString(body)}
	out, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}
	return pluginclient.Result{ExitCode: 0, Stdout: string(out)}
}

// --- newImportCmd properties (thin cobra wiring, no Execute()) ---

func TestNewImportCmd_Properties(t *testing.T) {
	tests := []struct {
		name        string
		flagName    string
		wantDefault string
	}{
		{name: "target flag defaults to both", flagName: "target", wantDefault: "both"},
		{name: "repo flag defaults to empty", flagName: "repo", wantDefault: ""},
		{name: "marketplace flag defaults to empty", flagName: "marketplace", wantDefault: ""},
		{name: "source flag defaults to empty", flagName: "source", wantDefault: ""},
	}

	cmd := newImportCmd()
	if cmd.Use != "import [skill...]" {
		t.Errorf("Use = %q, want %q", cmd.Use, "import [skill...]")
	}
	if cmd.Short == "" {
		t.Error("Short description is empty")
	}
	if cmd.RunE == nil {
		t.Error("RunE is nil")
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := cmd.Flags().Lookup(tt.flagName)
			if f == nil {
				t.Fatalf("flag %q not found", tt.flagName)
			}
			if f.DefValue != tt.wantDefault {
				t.Errorf("flag %q default = %q, want %q", tt.flagName, f.DefValue, tt.wantDefault)
			}
		})
	}

	yesFlag := cmd.Flags().Lookup("yes")
	if yesFlag == nil {
		t.Fatal("flag \"yes\" not found")
	}
	if yesFlag.Shorthand != "y" {
		t.Errorf("yes flag shorthand = %q, want %q", yesFlag.Shorthand, "y")
	}
	if yesFlag.DefValue != "false" {
		t.Errorf("yes flag default = %q, want %q", yesFlag.DefValue, "false")
	}
}

func TestParseRepo(t *testing.T) {
	tests := []struct {
		in        string
		wantOwner string
		wantName  string
		wantErr   bool
	}{
		{in: "octo/skills", wantOwner: "octo", wantName: "skills"},
		{in: "", wantErr: true},
		{in: "skills", wantErr: true},
		{in: "/skills", wantErr: true},
		{in: "octo/", wantErr: true},
		{in: "a/b/c", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			owner, name, err := parseRepo(tt.in)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if owner != tt.wantOwner || name != tt.wantName {
				t.Errorf("got %q/%q, want %q/%q", owner, name, tt.wantOwner, tt.wantName)
			}
		})
	}
}

func TestRunImportCmd_InvalidRepo(t *testing.T) {
	runner := &importCmdFakeRunner{}
	var stdout, stderr bytes.Buffer
	err := runImportCmd(context.Background(), runner, nil, []string{"x"}, "both", true, "", "", "", &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "invalid --repo") {
		t.Fatalf("err = %v, want containing %q", err, "invalid --repo")
	}
	if len(runner.calls) != 0 {
		t.Errorf("runner must not be called, got %v", runner.calls)
	}
}
