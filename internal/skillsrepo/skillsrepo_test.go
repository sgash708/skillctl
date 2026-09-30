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
			name: "正常系: 2つのskillをname昇順で返す",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "example-skill", "---\nname: example-skill\ndescription: スリープ抑止\n---\nbody\n")
				writeSkill(t, root, "another-skill", "---\nname: another-skill\ndescription: 別のskill\n---\nbody\n")
			},
			want: []Skill{
				{Name: "another-skill", Description: "別のskill", Dir: "another-skill"},
				{Name: "example-skill", Description: "スリープ抑止", Dir: "example-skill"},
			},
		},
		{
			name: "SKILL.mdが無いディレクトリは無視する",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "example-skill", "---\nname: example-skill\ndescription: スリープ抑止\n---\nbody\n")
				if err := os.MkdirAll(filepath.Join(root, "tools"), 0o755); err != nil {
					t.Fatal(err)
				}
			},
			want: []Skill{
				{Name: "example-skill", Description: "スリープ抑止", Dir: "example-skill"},
			},
		},
		{
			name: "隠しディレクトリは無視する",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, ".github", "---\nname: x\ndescription: y\n---\n")
			},
			want: []Skill{},
		},
		{
			name: "frontmatterの区切りが無ければエラー",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "name: broken\ndescription: no frontmatter\n")
			},
			wantErr: true,
		},
		{
			name: "descriptionが空ならエラー",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: broken\ndescription: \"\"\n---\n")
			},
			wantErr: true,
		},
		{
			name: "nameが空ならエラー",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: \"\"\ndescription: test\n---\n")
			},
			wantErr: true,
		},
		{
			name: "frontmatterが無効なYAMLならエラー",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\n[invalid: yaml: \n---\n")
			},
			wantErr: true,
		},
		{
			name: "frontmatterの閉じ区切りが無ければエラー",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: broken\ndescription: test\n")
			},
			wantErr: true,
		},
		{
			name: "descriptionの値に---という文字列が含まれていても閉じ区切りと誤認しない",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "dashes", "---\nname: dashes\ndescription: \"some steps --- like this\"\n---\nbody\n")
			},
			want: []Skill{
				{Name: "dashes", Description: "some steps --- like this", Dir: "dashes"},
			},
		},
		{
			name: "codex.dependencies.tools[].typeが空ならエラー",
			setup: func(t *testing.T, root string) {
				writeSkill(t, root, "broken", "---\nname: broken\ndescription: test\ncodex:\n  dependencies:\n    tools:\n      - value: github\n---\n")
			},
			wantErr: true,
		},
		{
			name: "codex.dependencies.tools[].valueが空ならエラー",
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
			name: "SKILL.md, README.md, .claude-plugin/plugin.jsonだけならOK",
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
			name: "generate未実行(.claude-pluginがまだ無い)でもOK",
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
			name: "skillディレクトリ直下に想定外のディレクトリ(hooks/)があればエラー",
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
			name: "skillディレクトリ直下に想定外のファイル(.mcp.json)があればエラー",
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
			name: ".claude-plugin配下に想定外のファイルがあればエラー",
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
			name: "SKILL.md, README.md, .claude-plugin/plugin.json, agents/openai.yamlだけならOK",
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
			name: "agents配下にopenai.yaml以外のファイルがあればエラー",
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
			name: "agentsがディレクトリではなくファイルならエラー",
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
			name: "assets配下に画像ファイル(.png, .svg)があればOK",
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
			name: "assets配下に画像以外のファイルがあればエラー",
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
			name: "assets配下にサブディレクトリがあればエラー",
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
			name: "assetsがディレクトリではなくファイルならエラー",
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
			name: "skillディレクトリ自体が読めなければエラー",
			setup: func(t *testing.T, root string) {
				// dirを作らない(Scanしないので存在しないケースを模す)
			},
			skills:  []Skill{{Name: "missing", Dir: "missing"}},
			wantErr: true,
		},
		{
			name: ".claude-pluginがディレクトリではなくファイルならエラー",
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
			name: ".claude-pluginが読めなければエラー",
			setup: func(t *testing.T, root string) {
				if os.Geteuid() == 0 {
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
			name: "assetsが読めなければエラー",
			setup: func(t *testing.T, root string) {
				if os.Geteuid() == 0 {
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
			name: "assets配下にシンボリックリンクがあればエラー",
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
