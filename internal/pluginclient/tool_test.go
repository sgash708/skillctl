package pluginclient

import (
	"reflect"
	"testing"
)

func TestClaudeTool_Args(t *testing.T) {
	c := ClaudeTool{}

	// Test Name separately since it returns string, not []string
	t.Run("Name", func(t *testing.T) {
		if got, want := c.Name(), "claude"; got != want {
			t.Errorf("Name() = %q, want %q", got, want)
		}
	})

	// Table-driven tests for []string methods
	tests := []struct {
		name string
		fn   func() []string
		want []string
	}{
		{
			name: "MarketplaceListArgs",
			fn:   c.MarketplaceListArgs,
			want: []string{"plugin", "marketplace", "list", "--json"},
		},
		{
			name: "MarketplaceAddArgs",
			fn: func() []string {
				return c.MarketplaceAddArgs("https://example.com/x")
			},
			want: []string{"plugin", "marketplace", "add", "https://example.com/x"},
		},
		{
			name: "InstallArgs",
			fn: func() []string {
				return c.InstallArgs("example-skill@example-skills")
			},
			want: []string{"plugin", "install", "example-skill@example-skills", "--yes", "--json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClaudeTool_ParseMarketplaceEntries(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    []MarketplaceEntry
		wantErr bool
	}{
		{
			name:   "github kindはrepoをsourceとして返す",
			stdout: `[{"name":"anthropic-agent-skills","source":"github","repo":"anthropics/skills"}]`,
			want:   []MarketplaceEntry{{Name: "anthropic-agent-skills", Source: "anthropics/skills"}},
		},
		{
			name:   "directory kindはpathをsourceとして返す",
			stdout: `[{"name":"example-skills","source":"directory","path":"/tmp/skills"}]`,
			want:   []MarketplaceEntry{{Name: "example-skills", Source: "/tmp/skills"}},
		},
		{
			name:   "git kindはurlをsourceとして返す",
			stdout: `[{"name":"example-skills","source":"git","url":"https://github.com/example-org/skills.git"}]`,
			want:   []MarketplaceEntry{{Name: "example-skills", Source: "https://github.com/example-org/skills.git"}},
		},
		{
			name:   "repo/path/urlのいずれも無ければsourceは空文字",
			stdout: `[{"name":"other"}]`,
			want:   []MarketplaceEntry{{Name: "other", Source: ""}},
		},
		{
			name:   "空配列",
			stdout: `[]`,
			want:   nil,
		},
		{
			name:    "不正なJSON",
			stdout:  `not json`,
			wantErr: true,
		},
	}
	c := ClaudeTool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.ParseMarketplaceEntries(tt.stdout)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestClaudeTool_ParseInstallResult(t *testing.T) {
	tests := []struct {
		name    string
		res     Result
		wantOK  bool
		wantMsg string
		wantErr bool
	}{
		{
			name:    "成功",
			res:     Result{ExitCode: 0, Stdout: `{"outcome":"ok","message":"Successfully installed plugin: example-skill@example-skills"}`},
			wantOK:  true,
			wantMsg: "Successfully installed plugin: example-skill@example-skills",
		},
		{
			name:    "既にinstall済み(冪等・成功扱い)",
			res:     Result{ExitCode: 0, Stdout: `{"outcome":"ok","message":"Plugin \"example-skill@example-skills\" is already installed (scope: user)"}`},
			wantOK:  true,
			wantMsg: `Plugin "example-skill@example-skills" is already installed (scope: user)`,
		},
		{
			name:    "失敗",
			res:     Result{ExitCode: 1, Stdout: `{"outcome":"failed","message":"Plugin not found"}`},
			wantOK:  false,
			wantMsg: "Plugin not found",
		},
		{
			name:    "不正なJSON",
			res:     Result{ExitCode: 0, Stdout: "not json"},
			wantErr: true,
		},
	}
	c := ClaudeTool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, msg, err := c.ParseInstallResult(tt.res)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.wantOK || msg != tt.wantMsg {
				t.Errorf("got (%v, %q), want (%v, %q)", ok, msg, tt.wantOK, tt.wantMsg)
			}
		})
	}
}

func TestCodexTool_Args(t *testing.T) {
	c := CodexTool{}

	// Test Name separately since it returns string, not []string
	t.Run("Name", func(t *testing.T) {
		if got, want := c.Name(), "codex"; got != want {
			t.Errorf("Name() = %q, want %q", got, want)
		}
	})

	// Table-driven tests for []string methods
	tests := []struct {
		name string
		fn   func() []string
		want []string
	}{
		{
			name: "MarketplaceListArgs",
			fn:   c.MarketplaceListArgs,
			want: []string{"plugin", "marketplace", "list", "--json"},
		},
		{
			name: "MarketplaceAddArgs",
			fn: func() []string {
				return c.MarketplaceAddArgs("https://example.com/x")
			},
			want: []string{"plugin", "marketplace", "add", "https://example.com/x", "--json"},
		},
		{
			name: "InstallArgs",
			fn: func() []string {
				return c.InstallArgs("example-skill@example-skills")
			},
			want: []string{"plugin", "add", "example-skill@example-skills", "--json"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCodexTool_ParseMarketplaceEntries(t *testing.T) {
	tests := []struct {
		name    string
		stdout  string
		want    []MarketplaceEntry
		wantErr bool
	}{
		{
			name:   "marketplaceSource.sourceをsourceとして返す",
			stdout: `{"marketplaces":[{"name":"example-skills","marketplaceSource":{"sourceType":"local","source":"/tmp/skills"}},{"name":"openai-curated"}]}`,
			want: []MarketplaceEntry{
				{Name: "example-skills", Source: "/tmp/skills"},
				{Name: "openai-curated", Source: ""},
			},
		},
		{
			name:    "不正なJSON",
			stdout:  `not json`,
			wantErr: true,
		},
	}
	c := CodexTool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := c.ParseMarketplaceEntries(tt.stdout)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestCodexTool_ParseInstallResult(t *testing.T) {
	tests := []struct {
		name    string
		res     Result
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "成功",
			res:    Result{ExitCode: 0, Stdout: `{"pluginId":"example-skill@example-skills"}`},
			wantOK: true,
		},
		{
			name:   "失敗(非ゼロ終了)",
			res:    Result{ExitCode: 1, Stderr: "not found"},
			wantOK: false,
		},
	}
	c := CodexTool{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, _, err := c.ParseInstallResult(tt.res)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
		})
	}
}
