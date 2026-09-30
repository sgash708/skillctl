package main

import (
	"bytes"
	"encoding/json"
	"github.com/sgash708/skillctl/internal/manifest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var testMeta = manifest.Meta{Name: "example-skills", Owner: "example-org"}

func writeSkillFixture(t *testing.T, root, dir string) {
	t.Helper()
	skillDir := filepath.Join(root, dir)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + dir + "\ndescription: desc-" + dir + "\n---\nbody\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func chdirTemp(t *testing.T, root string) {
	t.Helper()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("change directory: %v", err)
	}
	t.Cleanup(func() {
		os.Chdir(oldWd)
	})
}

func TestRunGenerate(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*testing.T, string) // prepares root before call
		check     bool                     // runGenerate(root, check)
		strict    bool                     // runGenerate(..., strict)
		wantStale bool                     // expect stale files returned?
		wantErr   bool                     // expect error?
		skipRoot  bool                     // skip: os.Geteuid() == 0
	}{
		{
			name: "write mode creates manifest files",
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "example-skill")
			},
			check:     false,
			wantStale: false,
			wantErr:   false,
		},
		{
			name: "check mode detects stale files before generation",
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "example-skill")
			},
			check:     true,
			wantStale: true,
			wantErr:   false,
		},
		{
			name: "check mode passes after write",
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "example-skill")
				// First write the manifests
				if _, err := runGenerate(root, testMeta, false, false); err != nil {
					t.Fatalf("initial write: %v", err)
				}
			},
			check:     true,
			wantStale: false,
			wantErr:   false,
		},
		{
			name: "scan error for missing root",
			setup: func(t *testing.T, root string) {
				// Don't create anything
			},
			check:     false,
			wantStale: false,
			wantErr:   true,
		},
		{
			name:      "unexpected file in skill directory blocks write mode",
			check:     false,
			strict:    true,
			wantStale: false,
			wantErr:   true,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "example-skill")
				hooksDir := filepath.Join(root, "example-skill", "hooks")
				if err := os.MkdirAll(hooksDir, 0o755); err != nil {
					t.Fatalf("create hooks dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte("{}"), 0o644); err != nil {
					t.Fatalf("write hooks.json: %v", err)
				}
			},
		},
		{
			name:      "unexpected file in skill directory blocks check mode",
			check:     true,
			strict:    true,
			wantStale: false,
			wantErr:   true,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "example-skill")
				hooksDir := filepath.Join(root, "example-skill", "hooks")
				if err := os.MkdirAll(hooksDir, 0o755); err != nil {
					t.Fatalf("create hooks dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(hooksDir, "hooks.json"), []byte("{}"), 0o644); err != nil {
					t.Fatalf("write hooks.json: %v", err)
				}
			},
		},
		{
			name:      "extra directories are allowed without strict",
			check:     false,
			strict:    false,
			wantStale: false,
			wantErr:   false,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "example-skill")
				scriptsDir := filepath.Join(root, "example-skill", "scripts")
				if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
					t.Fatalf("create scripts dir: %v", err)
				}
				if err := os.WriteFile(filepath.Join(scriptsDir, "run.sh"), []byte("#!/bin/sh\n"), 0o755); err != nil {
					t.Fatalf("write run.sh: %v", err)
				}
			},
		},
		{
			name:      "write error when .claude-plugin directory is read-only",
			skipRoot:  true,
			check:     false,
			wantStale: false,
			wantErr:   true,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "test-skill")
				// Create parent directory for .claude-plugin and make it read-only
				if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
					t.Fatalf("create .claude-plugin: %v", err)
				}
				if err := os.Chmod(filepath.Join(root, ".claude-plugin"), 0o555); err != nil {
					t.Fatalf("chmod .claude-plugin: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipRoot && os.Geteuid() == 0 {
				t.Skip("skipping permission test as root")
			}

			root := t.TempDir()
			tt.setup(t, root)

			// For missing root case, use a different root that doesn't exist
			testRoot := root
			if tt.name == "scan error for missing root" {
				testRoot = filepath.Join(root, "does-not-exist")
			}

			stale, err := runGenerate(testRoot, testMeta, tt.check, tt.strict)

			if tt.wantErr && err == nil {
				t.Errorf("runGenerate: wanted error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("runGenerate: %v", err)
			}

			if tt.wantStale && len(stale) == 0 {
				t.Error("wanted stale files, got none")
			}
			if !tt.wantStale && len(stale) != 0 {
				t.Errorf("wanted no stale files, got %v", stale)
			}

			// For write mode success, verify files were written
			if !tt.wantErr && !tt.check {
				if _, err := os.Stat(filepath.Join(testRoot, ".claude-plugin", "marketplace.json")); err != nil {
					t.Errorf("marketplace.json not written: %v", err)
				}
			}

			// Clean up read-only directories
			if tt.skipRoot {
				filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if info.IsDir() {
						os.Chmod(path, 0o755)
					}
					return nil
				})
			}
		})
	}
}

func TestNewGenerateCmd(t *testing.T) {
	tests := []struct {
		name         string
		check        bool
		setup        func(*testing.T, string) // prepares root before chdir
		wantErr      bool                     // expect Execute() to return error?
		wantStale    bool                     // expect stale files in error?
		assertStderr func(*testing.T, string) // optional: check stderr output
		skipRoot     bool                     // skip: os.Geteuid() == 0
	}{
		{
			name:      "write mode succeeds",
			check:     false,
			wantErr:   false,
			wantStale: false,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "test-skill")
			},
		},
		{
			name:      "check mode succeeds when no stale files",
			check:     true,
			wantErr:   false,
			wantStale: false,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "test-skill")
				// First write the manifests
				if _, err := runGenerate(root, testMeta, false, false); err != nil {
					t.Fatalf("initial write: %v", err)
				}
			},
		},
		{
			name:      "check mode fails with stale files and prints to stderr",
			check:     true,
			wantErr:   true,
			wantStale: true,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "test-skill")
			},
			assertStderr: func(t *testing.T, stderr string) {
				if !bytes.Contains([]byte(stderr), []byte("stale:")) {
					t.Errorf("stderr missing 'stale:' line, got: %s", stderr)
				}
				if !bytes.Contains([]byte(stderr), []byte("marketplace.json")) {
					t.Errorf("stderr missing marketplace.json path, got: %s", stderr)
				}
			},
		},
		{
			name:      "scan error propagates from RunE",
			check:     false,
			wantErr:   true,
			wantStale: false,
			setup: func(t *testing.T, root string) {
				// Write a skill with invalid YAML to trigger scan error
				skillDir := filepath.Join(root, "bad-skill")
				if err := os.MkdirAll(skillDir, 0o755); err != nil {
					t.Fatalf("create skill dir: %v", err)
				}
				// Invalid YAML (unclosed quote)
				content := "---\nname: 'bad\ndescription: test\n---\nbody\n"
				if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
					t.Fatalf("write SKILL.md: %v", err)
				}
			},
		},
		{
			name:      "write error when .claude-plugin directory is read-only",
			skipRoot:  true,
			check:     false,
			wantErr:   true,
			wantStale: false,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "test-skill")
				// Create parent directory for .claude-plugin and make it read-only
				if err := os.MkdirAll(filepath.Join(root, ".claude-plugin"), 0o755); err != nil {
					t.Fatalf("create .claude-plugin: %v", err)
				}
				if err := os.Chmod(filepath.Join(root, ".claude-plugin"), 0o555); err != nil {
					t.Fatalf("chmod .claude-plugin: %v", err)
				}
			},
		},
		{
			name:      "explicit root argument overrides current directory",
			check:     false,
			wantErr:   false,
			wantStale: false,
			setup: func(t *testing.T, root string) {
				writeSkillFixture(t, root, "explicit-root-skill")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipRoot && os.Geteuid() == 0 {
				t.Skip("skipping permission test as root")
			}

			root := t.TempDir()
			tt.setup(t, root)

			// For the explicit root argument test, stay in a different directory
			// to verify the root argument is actually used instead of current directory
			if tt.name == "explicit root argument overrides current directory" {
				otherDir := t.TempDir()
				chdirTemp(t, otherDir)
				// Set the explicit root argument
				cmd := newGenerateCmd()
				cmd.SetArgs([]string{root, "--owner", testMeta.Owner, "--name", testMeta.Name})
				cmd.Flags().Set("check", "false")

				// Capture stderr
				stderr := &bytes.Buffer{}
				cmd.SetErr(stderr)

				err := cmd.Execute()

				if tt.wantErr && err == nil {
					t.Errorf("Execute: wanted error, got none")
				}
				if !tt.wantErr && err != nil {
					t.Errorf("Execute: %v", err)
				}

				// Verify files were written to the explicit root, not current dir
				if !tt.wantErr {
					if _, err := os.Stat(filepath.Join(root, ".claude-plugin", "marketplace.json")); err != nil {
						t.Errorf("marketplace.json not written to explicit root: %v", err)
					}
					if _, err := os.Stat(filepath.Join(otherDir, ".claude-plugin", "marketplace.json")); err == nil {
						t.Error("marketplace.json should not be written to current directory")
					}
				}

				return
			}

			chdirTemp(t, root)

			cmd := newGenerateCmd()
			cmd.SetArgs([]string{"--owner", testMeta.Owner, "--name", testMeta.Name})
			cmd.Flags().Set("check", "false")
			if tt.check {
				cmd.Flags().Set("check", "true")
			}

			// Capture stderr
			stderr := &bytes.Buffer{}
			cmd.SetErr(stderr)

			err := cmd.Execute()

			if tt.wantErr && err == nil {
				t.Errorf("Execute: wanted error, got none")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Execute: %v", err)
			}

			if tt.assertStderr != nil {
				tt.assertStderr(t, stderr.String())
			}

			// Clean up read-only directories
			if tt.skipRoot {
				filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
					if info.IsDir() {
						os.Chmod(path, 0o755)
					}
					return nil
				})
			}
		})
	}
}

func TestNewGenerateCmd_Properties(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "command has correct properties and flag"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newGenerateCmd()

			if cmd.Use != "generate" {
				t.Errorf("Use = %q, want %q", cmd.Use, "generate")
			}
			if cmd.Short == "" {
				t.Error("Short description is empty")
			}
			if cmd.RunE == nil {
				t.Error("RunE is nil")
			}

			checkFlag := cmd.Flags().Lookup("check")
			if checkFlag == nil {
				t.Error("check flag not found")
			}
			if checkFlag.DefValue != "false" {
				t.Errorf("check flag default = %q, want %q", checkFlag.DefValue, "false")
			}
		})
	}
}

func TestNewGenerateCmd_OwnerRequired(t *testing.T) {
	root := t.TempDir()
	writeSkillFixture(t, root, "test-skill")

	cmd := newGenerateCmd()
	cmd.SetArgs([]string{root})
	cmd.SetErr(&bytes.Buffer{})

	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--owner is required") {
		t.Fatalf("err = %v, want containing %q", err, "--owner is required")
	}
	if _, err := os.Stat(filepath.Join(root, ".claude-plugin", "marketplace.json")); err == nil {
		t.Error("marketplace.json must not be written when --owner is missing")
	}
}

func TestNewGenerateCmd_NameFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantName string
	}{
		{name: "defaults to directory name", args: nil, wantName: "my-skills"},
		{name: "explicit --name wins", args: []string{"--name", "custom"}, wantName: "custom"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "my-skills")
			writeSkillFixture(t, root, "test-skill")

			cmd := newGenerateCmd()
			cmd.SetArgs(append([]string{root, "--owner", "example-org"}, tt.args...))
			cmd.SetErr(&bytes.Buffer{})
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute: %v", err)
			}

			data, err := os.ReadFile(filepath.Join(root, ".claude-plugin", "marketplace.json"))
			if err != nil {
				t.Fatal(err)
			}
			var mp manifest.Marketplace
			if err := json.Unmarshal(data, &mp); err != nil {
				t.Fatal(err)
			}
			if mp.Name != tt.wantName || mp.Owner.Name != "example-org" {
				t.Errorf("marketplace = %+v, want name %q owner %q", mp, tt.wantName, "example-org")
			}
		})
	}
}

func TestDefaultMarketplaceName(t *testing.T) {
	if got := defaultMarketplaceName("/tmp/foo/bar"); got != "bar" {
		t.Errorf("got %q, want %q", got, "bar")
	}
}

func TestNewGenerateCmd_StrictFlag(t *testing.T) {
	root := t.TempDir()
	writeSkillFixture(t, root, "example-skill")
	if err := os.MkdirAll(filepath.Join(root, "example-skill", "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}

	for _, strict := range []bool{false, true} {
		cmd := newGenerateCmd()
		args := []string{root, "--owner", "example-org"}
		if strict {
			args = append(args, "--strict")
		}
		cmd.SetArgs(args)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetOut(&bytes.Buffer{})
		err := cmd.Execute()
		if strict && err == nil {
			t.Error("--strict: wanted error for scripts/, got none")
		}
		if !strict && err != nil {
			t.Errorf("without --strict: %v", err)
		}
	}
}
