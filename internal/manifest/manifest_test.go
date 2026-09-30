package manifest

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sgash708/skillctl/internal/skillsrepo"
)

var testMeta = Meta{Name: "example-skills", Owner: "example-org"}

func sampleSkills() []skillsrepo.Skill {
	return []skillsrepo.Skill{
		{Name: "another-skill", Description: "別のskill", Dir: "another-skill"},
		{Name: "example-skill", Description: "スリープ抑止", Dir: "example-skill"},
	}
}

func TestBuildMarketplace(t *testing.T) {
	got := BuildMarketplace(testMeta, sampleSkills())

	if got.Name != testMeta.Name {
		t.Errorf("Name = %q, want %q", got.Name, testMeta.Name)
	}
	if got.Owner.Name != testMeta.Owner {
		t.Errorf("Owner.Name = %q, want %q", got.Owner.Name, testMeta.Owner)
	}
	want := []Plugin{
		{Name: "another-skill", Source: "./another-skill", Description: "別のskill", Version: PluginVersion},
		{Name: "example-skill", Source: "./example-skill", Description: "スリープ抑止", Version: PluginVersion},
	}
	if len(got.Plugins) != len(want) {
		t.Fatalf("len(Plugins) = %d, want %d", len(got.Plugins), len(want))
	}
	for i := range want {
		if got.Plugins[i] != want[i] {
			t.Errorf("Plugins[%d] = %+v, want %+v", i, got.Plugins[i], want[i])
		}
	}
}

func TestBuildPluginManifest(t *testing.T) {
	s := skillsrepo.Skill{Name: "example-skill", Description: "スリープ抑止", Dir: "example-skill"}
	got := BuildPluginManifest(s)
	want := PluginManifest{Name: "example-skill", Description: "スリープ抑止", Version: PluginVersion}
	if got != want {
		t.Errorf("got = %+v, want %+v", got, want)
	}
}

func TestBuildOpenAIYAML_NoCodexNoDisable(t *testing.T) {
	s := skillsrepo.Skill{Name: "plain-skill", Description: "desc", Dir: "plain-skill"}
	_, ok := BuildOpenAIYAML(s)
	if ok {
		t.Fatal("ok = true, want false when neither codex section nor disable-model-invocation is set")
	}
}

func TestBuildOpenAIYAML_DisableModelInvocationOnly(t *testing.T) {
	s := skillsrepo.Skill{Name: "plain-skill", Description: "desc", Dir: "plain-skill", DisableModelInvocation: true}
	got, ok := BuildOpenAIYAML(s)
	if !ok {
		t.Fatal("ok = false, want true when disable-model-invocation is set")
	}
	if got.Interface != nil {
		t.Errorf("Interface = %+v, want nil", got.Interface)
	}
	if got.Dependencies != nil {
		t.Errorf("Dependencies = %+v, want nil", got.Dependencies)
	}
	if got.Policy.AllowImplicitInvocation {
		t.Error("Policy.AllowImplicitInvocation = true, want false")
	}
}

func TestBuildOpenAIYAML_EmptyCodexSection(t *testing.T) {
	s := skillsrepo.Skill{
		Name:        "my-skill",
		Description: "desc",
		Dir:         "my-skill",
		Codex:       &skillsrepo.CodexConfig{},
	}
	got, ok := BuildOpenAIYAML(s)
	if !ok {
		t.Fatal("ok = false, want true when codex section is present (even if empty)")
	}
	if got.Interface != nil {
		t.Errorf("Interface = %+v, want nil when all codex interface fields are empty", got.Interface)
	}
	if got.Dependencies != nil {
		t.Errorf("Dependencies = %+v, want nil when no tools are set", got.Dependencies)
	}
}

func TestBuildOpenAIYAML_FullCodexConfig(t *testing.T) {
	s := skillsrepo.Skill{
		Name:        "my-skill",
		Description: "desc",
		Dir:         "my-skill",
		Codex: &skillsrepo.CodexConfig{
			DisplayName:      "My Skill",
			ShortDescription: "Short blurb",
			IconSmall:        "./assets/small.png",
			IconLarge:        "./assets/large.svg",
			BrandColor:       "#3B82F6",
			DefaultPrompt:    "Use $my-skill to do X.",
			Dependencies: []skillsrepo.CodexToolDependency{
				{Type: "mcp", Value: "github", Description: "GitHub MCP server", Transport: "streamable_http", URL: "https://api.githubcopilot.com/mcp/"},
			},
		},
	}
	got, ok := BuildOpenAIYAML(s)
	if !ok {
		t.Fatal("ok = false, want true when codex section is set")
	}
	if got.Interface == nil {
		t.Fatal("Interface = nil, want non-nil")
	}
	wantInterface := OpenAIInterface{
		DisplayName:      "My Skill",
		ShortDescription: "Short blurb",
		IconSmall:        "./assets/small.png",
		IconLarge:        "./assets/large.svg",
		BrandColor:       "#3B82F6",
		DefaultPrompt:    "Use $my-skill to do X.",
	}
	if *got.Interface != wantInterface {
		t.Errorf("Interface = %+v, want %+v", *got.Interface, wantInterface)
	}
	if got.Dependencies == nil {
		t.Fatal("Dependencies = nil, want non-nil")
	}
	wantTools := []OpenAITool{
		{Type: "mcp", Value: "github", Description: "GitHub MCP server", Transport: "streamable_http", URL: "https://api.githubcopilot.com/mcp/"},
	}
	if len(got.Dependencies.Tools) != len(wantTools) || got.Dependencies.Tools[0] != wantTools[0] {
		t.Errorf("Dependencies.Tools = %+v, want %+v", got.Dependencies.Tools, wantTools)
	}
	if !got.Policy.AllowImplicitInvocation {
		t.Error("Policy.AllowImplicitInvocation = false, want true (default)")
	}
}

func TestWriteAndCheck_OpenAIYAML(t *testing.T) {
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
		{Name: "plain-skill", Description: "desc2", Dir: "plain-skill"},
	}
	for _, s := range skills {
		if err := os.MkdirAll(filepath.Join(root, s.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	written, err := Write(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	foundOpenAIYAML := false
	for _, w := range written {
		if w == filepath.Join("my-skill", "agents", "openai.yaml") {
			foundOpenAIYAML = true
		}
		if w == filepath.Join("plain-skill", "agents", "openai.yaml") {
			t.Errorf("plain-skill should not get agents/openai.yaml, but Write reported %q", w)
		}
	}
	if !foundOpenAIYAML {
		t.Fatalf("expected my-skill/agents/openai.yaml to be written, got %v", written)
	}

	if _, err := os.Stat(filepath.Join(root, "plain-skill", "agents", "openai.yaml")); !os.IsNotExist(err) {
		t.Fatalf("plain-skill/agents/openai.yaml should not exist, stat err = %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(root, "my-skill", "agents", "openai.yaml"))
	if err != nil {
		t.Fatalf("read agents/openai.yaml: %v", err)
	}
	if !strings.Contains(string(raw), "allow_implicit_invocation: false") {
		t.Errorf("agents/openai.yaml content = %q, want it to contain allow_implicit_invocation: false", raw)
	}

	stale, err := Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check after Write: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("Check reported stale files right after Write: %v", stale)
	}

	skills[0].DisableModelInvocation = false
	stale, err = Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check after change: %v", err)
	}
	if len(stale) == 0 {
		t.Fatal("Check should report stale agents/openai.yaml after disable-model-invocation change")
	}

	written2, err := Write(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Write after disable-model-invocation removed: %v", err)
	}
	for _, w := range written2 {
		if w == filepath.Join("my-skill", "agents", "openai.yaml") {
			t.Fatalf("agents/openai.yaml should not be reported as written once disable-model-invocation is removed, got %v", written2)
		}
	}
	if _, err := os.Stat(filepath.Join(root, "my-skill", "agents", "openai.yaml")); !os.IsNotExist(err) {
		t.Fatalf("agents/openai.yaml should be removed once disable-model-invocation is removed, stat err = %v", err)
	}

	stale, err = Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check after removal: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("Check should report no stale files after regenerating, got %v", stale)
	}
}

func TestWrite_OpenAIYAMLMarshalError(t *testing.T) {
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
	}
	if err := os.MkdirAll(filepath.Join(root, skills[0].Dir), 0o755); err != nil {
		t.Fatal(err)
	}

	origYAMLMarshal := yamlMarshal
	defer func() { yamlMarshal = origYAMLMarshal }()
	testErr := errors.New("yaml marshal error")
	yamlMarshal = func(v interface{}) ([]byte, error) { return nil, testErr }

	if _, err := Write(root, testMeta, skills); err != testErr {
		t.Fatalf("Write: expected testErr, got %v", err)
	}
}

func TestWrite_OpenAIYAMLMkdirAllError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission test not meaningful")
	}
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
	}
	skillDir := filepath.Join(root, skills[0].Dir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// "agents" という名前のファイルを事前に置き、MkdirAll(agents/)を失敗させる
	if err := os.WriteFile(filepath.Join(skillDir, "agents"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Write(root, testMeta, skills); err == nil {
		t.Fatal("expected Write to fail when agents/ cannot be created")
	}
}

func TestWrite_OpenAIYAMLWriteFileError(t *testing.T) {
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
	}
	skillDir := filepath.Join(root, skills[0].Dir)
	// openai.yaml をディレクトリとして事前に作っておき、WriteFileを失敗させる
	if err := os.MkdirAll(filepath.Join(skillDir, "agents", "openai.yaml"), 0o755); err != nil {
		t.Fatal(err)
	}

	if _, err := Write(root, testMeta, skills); err == nil {
		t.Fatal("expected Write to fail when agents/openai.yaml path is a directory")
	}
}

func TestWrite_OpenAIYAMLRemoveError(t *testing.T) {
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "plain-skill", Description: "desc", Dir: "plain-skill"},
	}
	skillDir := filepath.Join(root, skills[0].Dir)
	// openai.yaml を空でないディレクトリにしておき、生成対象外時のos.Removeを失敗させる
	oaiDir := filepath.Join(skillDir, "agents", "openai.yaml")
	if err := os.MkdirAll(oaiDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(oaiDir, "nested.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := Write(root, testMeta, skills); err == nil {
		t.Fatal("expected Write to fail when agents/openai.yaml cannot be removed")
	}
}

func TestCheck_OpenAIYAMLMissing(t *testing.T) {
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
	}
	if err := os.MkdirAll(filepath.Join(root, skills[0].Dir), 0o755); err != nil {
		t.Fatal(err)
	}

	stale, err := Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := filepath.Join("my-skill", "agents", "openai.yaml")
	found := false
	for _, s := range stale {
		if s == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q to be reported stale, got %v", want, stale)
	}
}

func TestCheck_OpenAIYAMLUnreadableWhenNotWanted(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission test not meaningful")
	}
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "plain-skill", Description: "desc", Dir: "plain-skill"},
	}
	agentsDir := filepath.Join(root, skills[0].Dir, "agents")
	if err := os.MkdirAll(agentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	oaiPath := filepath.Join(agentsDir, "openai.yaml")
	if err := os.WriteFile(oaiPath, []byte("policy: {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(oaiPath, 0o000); err != nil {
		t.Skipf("could not chmod: %v", err)
	}
	defer os.Chmod(oaiPath, 0o644)

	if _, err := Check(root, testMeta, skills); err == nil {
		t.Fatal("expected a real error, not silently treated as stale")
	}
}

func TestCheck_OpenAIYAMLUnreadableIsARealError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission test not meaningful")
	}
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
	}
	if err := os.MkdirAll(filepath.Join(root, skills[0].Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, testMeta, skills); err != nil {
		t.Fatalf("Write: %v", err)
	}

	oaiPath := filepath.Join(root, skills[0].Dir, "agents", "openai.yaml")
	if err := os.Chmod(oaiPath, 0o000); err != nil {
		t.Skipf("could not chmod: %v", err)
	}
	defer os.Chmod(oaiPath, 0o644)

	stale, err := Check(root, testMeta, skills)
	if err == nil {
		t.Fatalf("expected a real error, got stale=%v, err=nil", stale)
	}
}

func TestCheck_OpenAIYAMLMarshalError(t *testing.T) {
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
	}
	if err := os.MkdirAll(filepath.Join(root, skills[0].Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, testMeta, skills); err != nil {
		t.Fatalf("Write: %v", err)
	}

	origYAMLMarshal := yamlMarshal
	defer func() { yamlMarshal = origYAMLMarshal }()
	testErr := errors.New("yaml marshal error")
	yamlMarshal = func(v interface{}) ([]byte, error) { return nil, testErr }

	if _, err := Check(root, testMeta, skills); err != testErr {
		t.Fatalf("Check: expected testErr, got %v", err)
	}
}

func TestCheck_OpenAIYAMLContentMismatch(t *testing.T) {
	root := t.TempDir()
	skills := []skillsrepo.Skill{
		{Name: "my-skill", Description: "desc", Dir: "my-skill", DisableModelInvocation: true},
	}
	if err := os.MkdirAll(filepath.Join(root, skills[0].Dir), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Write(root, testMeta, skills); err != nil {
		t.Fatalf("Write: %v", err)
	}

	oaiPath := filepath.Join(root, skills[0].Dir, "agents", "openai.yaml")
	if err := os.WriteFile(oaiPath, []byte("policy:\n  allow_implicit_invocation: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale, err := Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	want := filepath.Join("my-skill", "agents", "openai.yaml")
	found := false
	for _, s := range stale {
		if s == want {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected %q to be reported stale, got %v", want, stale)
	}
}

func TestWriteAndCheck(t *testing.T) {
	root := t.TempDir()
	skills := sampleSkills()
	for _, s := range skills {
		if err := os.MkdirAll(filepath.Join(root, s.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	written, err := Write(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	wantWritten := []string{
		".claude-plugin/marketplace.json",
		"another-skill/.claude-plugin/plugin.json",
		"example-skill/.claude-plugin/plugin.json",
	}
	if len(written) != len(wantWritten) {
		t.Fatalf("len(written) = %d, want %d (got=%v)", len(written), len(wantWritten), written)
	}

	raw, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "marketplace.json"))
	if err != nil {
		t.Fatalf("read marketplace.json: %v", err)
	}
	var mp Marketplace
	if err := json.Unmarshal(raw, &mp); err != nil {
		t.Fatalf("unmarshal marketplace.json: %v", err)
	}
	if mp.Name != testMeta.Name {
		t.Errorf("marketplace.json Name = %q, want %q", mp.Name, testMeta.Name)
	}

	stale, err := Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check after Write: %v", err)
	}
	if len(stale) != 0 {
		t.Fatalf("Check reported stale files right after Write: %v", stale)
	}

	// マニフェスト生成後にskillの説明が変わったら stale として検出される
	skills[0].Description = "更新後の説明"
	stale, err = Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check after change: %v", err)
	}
	if len(stale) == 0 {
		t.Fatal("Check should report stale files after description change")
	}
}

func TestCheck_UnreadableFileIsARealErrorNotStale(t *testing.T) {
	// Check must distinguish "file doesn't exist yet" (genuinely stale) from
	// "file exists but can't be read" (a real I/O error; `generate` won't fix it,
	// and reporting it as merely "stale" would be misleading).
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission test not meaningful")
	}

	tests := []struct {
		name      string
		chmodPath func(root string) string
	}{
		{
			name: "marketplace.json is unreadable",
			chmodPath: func(root string) string {
				return filepath.Join(root, ".claude-plugin", "marketplace.json")
			},
		},
		{
			name: "a plugin.json is unreadable",
			chmodPath: func(root string) string {
				return filepath.Join(root, sampleSkills()[0].Dir, ".claude-plugin", "plugin.json")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			skills := sampleSkills()
			for _, s := range skills {
				if err := os.MkdirAll(filepath.Join(root, s.Dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := Write(root, testMeta, skills); err != nil {
				t.Fatalf("Write: %v", err)
			}

			target := tt.chmodPath(root)
			if err := os.Chmod(target, 0o000); err != nil {
				t.Skipf("could not chmod: %v", err)
			}
			defer os.Chmod(target, 0o644)

			stale, err := Check(root, testMeta, skills)
			if err == nil {
				t.Fatalf("expected a real error, got stale=%v, err=nil", stale)
			}
			if len(stale) != 0 {
				t.Errorf("unreadable file should not be silently folded into stale list, got %v", stale)
			}
		})
	}
}

func TestCheck_MissingManifests(t *testing.T) {
	root := t.TempDir()
	skills := sampleSkills()
	for _, s := range skills {
		if err := os.MkdirAll(filepath.Join(root, s.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	stale, err := Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if len(stale) != 3 {
		t.Fatalf("len(stale) = %d, want 3 (got=%v)", len(stale), stale)
	}
}

func TestWrite_FileConflict(t *testing.T) {
	root := t.TempDir()
	skills := sampleSkills()
	for _, s := range skills {
		if err := os.MkdirAll(filepath.Join(root, s.Dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a file where we need a directory
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin"), []byte("conflict"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Write(root, testMeta, skills)
	if err == nil {
		t.Fatal("Expected Write to fail when file conflicts with directory path, got nil")
	}
}

func TestCheck_MismatchedContent(t *testing.T) {
	// This test ensures Check detects when file content doesn't match expected content
	root := t.TempDir()
	skills := sampleSkills()
	for _, s := range skills {
		if err := os.MkdirAll(filepath.Join(root, s.Dir, ".claude-plugin"), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	// Write mismatched content to marketplace.json
	if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".claude-plugin", "marketplace.json"), []byte("not matching"), 0o644); err != nil {
		t.Fatal(err)
	}

	stale, err := Check(root, testMeta, skills)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	// Should detect the file as stale (content doesn't match)
	if len(stale) == 0 {
		t.Fatal("Check should report stale files when content mismatches")
	}
}

func TestWrite_PermissionErrors(t *testing.T) {
	// Skip this test if running as root (permission bits don't restrict root)
	if os.Geteuid() == 0 {
		t.Skip("running as root, permission test not meaningful")
	}

	tests := []struct {
		name      string
		chmodPath func(root string) string
		desc      string
	}{
		{
			name: "root directory read-only",
			chmodPath: func(root string) string {
				return root
			},
			desc: "Expected Write to fail on read-only directory, got nil",
		},
		{
			name: "skill directory read-only",
			chmodPath: func(root string) string {
				return filepath.Join(root, sampleSkills()[0].Dir)
			},
			desc: "Expected Write to fail when unable to write plugin files, got nil",
		},
		{
			name: "plugin directory read-only",
			chmodPath: func(root string) string {
				pluginDir := filepath.Join(root, sampleSkills()[0].Dir, ".claude-plugin")
				if err := os.MkdirAll(pluginDir, 0o755); err != nil {
					t.Fatal(err)
				}
				return pluginDir
			},
			desc: "Expected Write to fail when plugin directory is read-only, got nil",
		},
		{
			name: "root plugin directory read-only",
			chmodPath: func(root string) string {
				rootPluginDir := filepath.Join(root, ".claude-plugin")
				if err := os.MkdirAll(rootPluginDir, 0o755); err != nil {
					t.Fatal(err)
				}
				return rootPluginDir
			},
			desc: "Expected Write to fail when root plugin directory is read-only, got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			skills := sampleSkills()
			for _, s := range skills {
				if err := os.MkdirAll(filepath.Join(root, s.Dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			// Also ensure root .claude-plugin dir exists for some tests
			if tt.name == "skill directory read-only" {
				if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			chmodPath := tt.chmodPath(root)
			if err := os.Chmod(chmodPath, 0o444); err != nil {
				t.Skipf("could not chmod: %v", err)
			}
			defer os.Chmod(chmodPath, 0o755)

			_, err := Write(root, testMeta, skills)
			if err == nil {
				t.Fatal(tt.desc)
			}
		})
	}
}

func TestMarshalIndentErrors(t *testing.T) {
	tests := []struct {
		name       string
		fn         func(root string, meta Meta, skills []skillsrepo.Skill) ([]string, error)
		failOnCall int
		desc       string
	}{
		{
			name:       "Write marketplace error",
			fn:         Write,
			failOnCall: 1,
			desc:       "Expected Write to fail when jsonMarshalIndent errors, got nil",
		},
		{
			name:       "Write loop error",
			fn:         Write,
			failOnCall: 2,
			desc:       "Expected Write to fail when jsonMarshalIndent errors in loop, got nil",
		},
		{
			name:       "Check marketplace error",
			fn:         Check,
			failOnCall: 1,
			desc:       "Expected Check to fail when jsonMarshalIndent errors, got nil",
		},
		{
			name:       "Check loop error",
			fn:         Check,
			failOnCall: 2,
			desc:       "Expected Check to fail when jsonMarshalIndent errors in loop, got nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			skills := sampleSkills()
			for _, s := range skills {
				if err := os.MkdirAll(filepath.Join(root, s.Dir), 0o755); err != nil {
					t.Fatal(err)
				}
			}

			origMarshal := jsonMarshalIndent
			defer func() { jsonMarshalIndent = origMarshal }()

			testErr := errors.New("marshal error")
			callCount := 0
			jsonMarshalIndent = func(v interface{}, prefix, indent string) ([]byte, error) {
				callCount++
				if callCount >= tt.failOnCall {
					return nil, testErr
				}
				return origMarshal(v, prefix, indent)
			}

			_, err := tt.fn(root, testMeta, skills)
			if err == nil {
				t.Fatal(tt.desc)
			}
			if err != testErr {
				t.Errorf("Expected testErr, got %v", err)
			}
		})
	}
}
