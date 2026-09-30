package skillsrepo

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func writeSkill(t *testing.T, root, dir, content string) {
	t.Helper()
	skillDir := filepath.Join(root, dir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScan(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string)
		want    []Skill
		wantErr bool
	}{
		{
			name: "happy path: returns two skills sorted by name ascending",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "example-skill", "---\nname: example-skill\ndescription: Sleep prevention\n---\nbody\n")
				writeSkill(t, root, "another-skill", "---\nname: another-skill\ndescription: Another skill\n---\nbody\n")
			},
			want: []Skill{
				{Name: "another-skill", Description: "Another skill", Dir: "another-skill"},
				{Name: "example-skill", Description: "Sleep prevention", Dir: "example-skill"},
			},
		},
		{
			name: "ignores directories without SKILL.md",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "example-skill", "---\nname: example-skill\ndescription: Sleep prevention\n---\nbody\n")
				if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: []Skill{
				{Name: "example-skill", Description: "Sleep prevention", Dir: "example-skill"},
			},
		},
		{
			name: "ignores hidden directories",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, ".github", "---\nname: x\ndescription: y\n---\n")
			},
			want: []Skill{},
		},
		{
			name: "errors if the frontmatter delimiter is missing",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "name: broken\ndescription: no frontmatter\n")
			},
			wantErr: true,
		},
		{
			name: "errors if description is empty",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: broken\ndescription: \"\"\n---\n")
			},
			wantErr: true,
		},
		{
			name: "errors if name is empty",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: \"\"\ndescription: test\n---\n")
			},
			wantErr: true,
		},
		{
			name: "errors if the frontmatter is invalid YAML",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\n[invalid: yaml: \n---\n")
			},
			wantErr: true,
		},
		{
			name: "errors if the frontmatter closing delimiter is missing",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: broken\ndescription: test\n")
			},
			wantErr: true,
		},
		{
			name: "does not mistake a --- inside the description value for the closing delimiter",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "dashes", "---\nname: dashes\ndescription: \"some steps --- like this\"\n---\nbody\n")
			},
			want: []Skill{
				{Name: "dashes", Description: "some steps --- like this", Dir: "dashes"},
			},
		},
		{
			name: "errors if codex.dependencies.tools[].type is empty",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: broken\ndescription: test\ncodex:\n  dependencies:\n    tools:\n      - value: github\n---\n")
			},
			wantErr: true,
		},
		{
			name: "errors if codex.dependencies.tools[].value is empty",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: broken\ndescription: test\ncodex:\n  dependencies:\n    tools:\n      - type: mcp\n---\n")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			got, err := Scan(root)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("len(got) = %d, want %d (got=%+v)", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestScan_CodexFrontmatter(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "my-skill", `---
name: my-skill
description: something
disable-model-invocation: true
codex:
  display_name: "My Skill"
  short_description: "Short blurb"
  icon_small: "./assets/small.png"
  icon_large: "./assets/large.svg"
  brand_color: "#3B82F6"
  default_prompt: "Use $my-skill to do X."
  dependencies:
    tools:
      - type: mcp
        value: github
        description: "GitHub MCP server"
        transport: streamable_http
        url: "https://api.githubcopilot.com/mcp/"
---
body
`)

	got, err := Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(got) = %d, want 1", len(got))
	}
	s := got[0]
	if !s.DisableModelInvocation {
		t.Error("DisableModelInvocation = false, want true")
	}
	if s.Codex == nil {
		t.Fatal("Codex = nil, want non-nil")
	}
	want := CodexConfig{
		DisplayName:      "My Skill",
		ShortDescription: "Short blurb",
		IconSmall:        "./assets/small.png",
		IconLarge:        "./assets/large.svg",
		BrandColor:       "#3B82F6",
		DefaultPrompt:    "Use $my-skill to do X.",
		Dependencies: []CodexToolDependency{
			{Type: "mcp", Value: "github", Description: "GitHub MCP server", Transport: "streamable_http", URL: "https://api.githubcopilot.com/mcp/"},
		},
	}
	if !reflect.DeepEqual(*s.Codex, want) {
		t.Errorf("Codex = %+v, want %+v", *s.Codex, want)
	}
}

func TestScan_NoCodexSection(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root, "plain-skill", "---\nname: plain-skill\ndescription: desc\n---\n")

	got, err := Scan(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got[0].DisableModelInvocation {
		t.Error("DisableModelInvocation = true, want false (default)")
	}
	if got[0].Codex != nil {
		t.Errorf("Codex = %+v, want nil", got[0].Codex)
	}
}

func TestScan_RootNotExist(t *testing.T) {
	_, err := Scan("/path/does/not/exist")
	if err == nil {
		t.Fatal("expected error for missing root")
	}
}

func TestScan_ReadError(t *testing.T) {
	if cannotTestPermissions() {
		t.Skip("cannot provoke permission errors here")
	}
	root := t.TempDir()
	skillDir := filepath.Join(root, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("test"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Remove read permissions
	if err := os.Chmod(skillFile, 0o000); err != nil {
		t.Fatal(err)
	}

	_, err := Scan(root)
	if err == nil {
		t.Fatal("expected error for unreadable SKILL.md")
	}
}

func TestCheckAllowedContents(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, root string)
		skills  []Skill
		wantErr bool
	}{
		{
			name: "OK with only SKILL.md, README.md, .claude-plugin/plugin.json",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, ".claude-plugin"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("readme"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "plugin.json"), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills: []Skill{{Name: "example-skill", Dir: "example-skill"}},
		},
		{
			name: "OK even if generate has not run (.claude-plugin does not exist yet)",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills: []Skill{{Name: "example-skill", Dir: "example-skill"}},
		},
		{
			name: "errors if there is an unexpected directory (hooks/) directly under the skill directory",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, "hooks"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "hooks", "hooks.json"), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if there is an unexpected file (.mcp.json) directly under the skill directory",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".mcp.json"), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if there is an unexpected file under .claude-plugin",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, ".claude-plugin"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "plugin.json"), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".claude-plugin", "extra.json"), []byte("{}"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "OK with only SKILL.md, README.md, .claude-plugin/plugin.json, agents/openai.yaml",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "agents", "openai.yaml"), []byte("policy: {}"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills: []Skill{{Name: "example-skill", Dir: "example-skill"}},
		},
		{
			name: "errors if there is a file other than openai.yaml under agents",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, "agents"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "agents", "extra.txt"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if agents is a file rather than a directory",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "agents"), []byte("not a dir"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "OK if there are image files (.png, .svg) under assets",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "assets", "small.png"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "assets", "large.svg"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills: []Skill{{Name: "example-skill", Dir: "example-skill"}},
		},
		{
			name: "errors if there is a non-image file under assets",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "assets", "run.sh"), []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if there is a subdirectory under assets",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, "assets", "nested"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if assets is a file rather than a directory",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "assets"), []byte("not a dir"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if the skill directory itself cannot be read",
			setup: func(t *testing.T, root string) {
				// Do not create dir (simulates a non-existent case since we do not Scan)
			},
			skills:  []Skill{{Name: "missing", Dir: "missing"}},
			wantErr: true,
		},
		{
			name: "errors if .claude-plugin is a file rather than a directory",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(dir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, ".claude-plugin"), []byte("not a dir"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if .claude-plugin cannot be read",
			setup: func(t *testing.T, root string) {
				if cannotTestPermissions() {
					t.Skip("running as root, permission test not meaningful")
				}
				dir := filepath.Join(root, "example-skill")
				pluginDir := filepath.Join(dir, ".claude-plugin")
				if err := os.MkdirAll(pluginDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(pluginDir, 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(pluginDir, 0o755) })
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if assets cannot be read",
			setup: func(t *testing.T, root string) {
				if cannotTestPermissions() {
					t.Skip("running as root, permission test not meaningful")
				}
				dir := filepath.Join(root, "example-skill")
				assetsDir := filepath.Join(dir, "assets")
				if err := os.MkdirAll(assetsDir, 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(assetsDir, 0o000); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { os.Chmod(assetsDir, 0o755) })
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
		{
			name: "errors if there is a symlink under assets",
			setup: func(t *testing.T, root string) {
				dir := filepath.Join(root, "example-skill")
				if err := os.MkdirAll(filepath.Join(dir, "assets"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("skill"), 0o644); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(root, "outside.png")
				if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, filepath.Join(dir, "assets", "small.png")); err != nil {
					t.Fatal(err)
				}
			},
			skills:  []Skill{{Name: "example-skill", Dir: "example-skill"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			tt.setup(t, root)

			err := CheckAllowedContents(root, tt.skills)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
