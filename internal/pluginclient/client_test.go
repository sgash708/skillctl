package pluginclient

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeCall struct {
	name string
	args []string
}

type fakeRunner struct {
	calls   []fakeCall
	results map[string]Result // key: strings.Join(args, " ")
	errFor  map[string]error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (Result, error) {
	f.calls = append(f.calls, fakeCall{name: name, args: args})
	key := name + " " + joinArgs(args)
	if err, ok := f.errFor[key]; ok {
		return Result{ExitCode: -1}, err
	}
	if res, ok := f.results[key]; ok {
		return res, nil
	}
	return Result{}, nil
}

func joinArgs(args []string) string {
	out := ""
	for i, a := range args {
		if i > 0 {
			out += " "
		}
		out += a
	}
	return out
}

func TestClient_EnsureMarketplace(t *testing.T) {
	tests := []struct {
		name      string
		listRes   Result
		wantAdded bool
		wantErr   bool
		wantErrIs string // substring expected in the error, if wantErr
	}{
		{
			name:      "adds if not registered",
			listRes:   Result{ExitCode: 0, Stdout: `[]`},
			wantAdded: true,
		},
		{
			name:      "does not add if registered with the same source",
			listRes:   Result{ExitCode: 0, Stdout: `[{"name":"example-skills","source":"git","url":"https://example.com/skills"}]`},
			wantAdded: false,
		},
		{
			name:      "does not add if registered and the source differs only by a trailing / or .git (treated as the same source)",
			listRes:   Result{ExitCode: 0, Stdout: `[{"name":"example-skills","source":"git","url":"https://example.com/skills.git"}]`},
			wantAdded: false,
		},
		{
			name:      "does not add if registered but there is no source info to decide on",
			listRes:   Result{ExitCode: 0, Stdout: `[{"name":"example-skills"}]`},
			wantAdded: false,
		},
		{
			name:      "errors if registered with a different source (no automatic add/replace)",
			listRes:   Result{ExitCode: 0, Stdout: `[{"name":"example-skills","source":"directory","path":"/tmp/local-dev"}]`},
			wantAdded: false,
			wantErr:   true,
			wantErrIs: "already registered from a different source",
		},
		{
			name:      "looks only at the target name even if other marketplaces are mixed in, and does not add if the source is the same",
			listRes:   Result{ExitCode: 0, Stdout: `[{"name":"other-marketplace","source":"github","repo":"other/repo"},{"name":"example-skills","source":"git","url":"https://example.com/skills"}]`},
			wantAdded: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ClaudeTool{}
			fr := &fakeRunner{
				results: map[string]Result{
					"claude " + joinArgs(tool.MarketplaceListArgs()):                            tt.listRes,
					"claude " + joinArgs(tool.MarketplaceAddArgs("https://example.com/skills")): {ExitCode: 0},
				},
			}
			c := NewClient(tool, fr)

			err := c.EnsureMarketplace(context.Background(), "example-skills", "https://example.com/skills")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrIs != "" && !strings.Contains(err.Error(), tt.wantErrIs) {
					t.Errorf("err = %q, want containing %q", err.Error(), tt.wantErrIs)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			addCalled := false
			for _, call := range fr.calls {
				if reflect.DeepEqual(call.args, tool.MarketplaceAddArgs("https://example.com/skills")) {
					addCalled = true
				}
			}
			if addCalled != tt.wantAdded {
				t.Errorf("add called = %v, want %v", addCalled, tt.wantAdded)
			}
		})
	}
}

func TestClient_EnsureMarketplace_ListFails(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "list fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ClaudeTool{}
			fr := &fakeRunner{
				errFor: map[string]error{
					"claude " + joinArgs(tool.MarketplaceListArgs()): errors.New("binary not found"),
				},
			}
			c := NewClient(tool, fr)

			err := c.EnsureMarketplace(context.Background(), "example-skills", "https://example.com/skills")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestClient_EnsureMarketplace_ParseFails(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "parse marketplace names fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ClaudeTool{}
			fr := &fakeRunner{
				results: map[string]Result{
					"claude " + joinArgs(tool.MarketplaceListArgs()): {ExitCode: 0, Stdout: "invalid json"},
				},
			}
			c := NewClient(tool, fr)

			err := c.EnsureMarketplace(context.Background(), "example-skills", "https://example.com/skills")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestClient_EnsureMarketplace_AddFails(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "marketplace add runner fails",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ClaudeTool{}
			fr := &fakeRunner{
				results: map[string]Result{
					"claude " + joinArgs(tool.MarketplaceListArgs()): {ExitCode: 0, Stdout: `[]`},
				},
				errFor: map[string]error{
					"claude " + joinArgs(tool.MarketplaceAddArgs("https://example.com/skills")): errors.New("network error"),
				},
			}
			c := NewClient(tool, fr)

			err := c.EnsureMarketplace(context.Background(), "example-skills", "https://example.com/skills")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestClient_EnsureMarketplace_AddExitCodeNonZero(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "marketplace add exit code non-zero",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ClaudeTool{}
			fr := &fakeRunner{
				results: map[string]Result{
					"claude " + joinArgs(tool.MarketplaceListArgs()):                            {ExitCode: 0, Stdout: `[]`},
					"claude " + joinArgs(tool.MarketplaceAddArgs("https://example.com/skills")): {ExitCode: 1, Stderr: "failed to add"},
				},
			}
			c := NewClient(tool, fr)

			err := c.EnsureMarketplace(context.Background(), "example-skills", "https://example.com/skills")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

func TestClient_Install(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "install succeeds",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ClaudeTool{}
			fr := &fakeRunner{
				results: map[string]Result{
					"claude " + joinArgs(tool.InstallArgs("example-skill@example-skills")): {
						ExitCode: 0,
						Stdout:   `{"outcome":"ok","message":"Successfully installed plugin: example-skill@example-skills"}`,
					},
				},
			}
			c := NewClient(tool, fr)

			ok, msg, err := c.Install(context.Background(), "example-skill@example-skills")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !ok {
				t.Error("ok = false, want true")
			}
			if msg != "Successfully installed plugin: example-skill@example-skills" {
				t.Errorf("message = %q", msg)
			}
		})
	}
}

func TestClient_Install_RunnerError(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "install runner error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := ClaudeTool{}
			fr := &fakeRunner{
				errFor: map[string]error{
					"claude " + joinArgs(tool.InstallArgs("example-skill@example-skills")): errors.New("binary not found"),
				},
			}
			c := NewClient(tool, fr)

			_, _, err := c.Install(context.Background(), "example-skill@example-skills")
			if err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}
