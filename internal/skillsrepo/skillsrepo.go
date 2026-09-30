package skillsrepo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name                   string
	Description            string
	Dir                    string
	DisableModelInvocation bool
	Codex                  *CodexConfig
}

// CodexConfig はSKILL.mdのfrontmatterにある`codex:`セクションから読み取った、
// Codex固有のagents/openai.yaml生成用メタデータ。
type CodexConfig struct {
	DisplayName      string
	ShortDescription string
	IconSmall        string
	IconLarge        string
	BrandColor       string
	DefaultPrompt    string
	Dependencies     []CodexToolDependency
}

type CodexToolDependency struct {
	Type        string
	Value       string
	Description string
	Transport   string
	URL         string
}

type codexFrontmatter struct {
	DisplayName      string                      `yaml:"display_name"`
	ShortDescription string                      `yaml:"short_description"`
	IconSmall        string                      `yaml:"icon_small"`
	IconLarge        string                      `yaml:"icon_large"`
	BrandColor       string                      `yaml:"brand_color"`
	DefaultPrompt    string                      `yaml:"default_prompt"`
	Dependencies     *codexDependencyFrontmatter `yaml:"dependencies"`
}

type codexDependencyFrontmatter struct {
	Tools []codexToolFrontmatter `yaml:"tools"`
}

type codexToolFrontmatter struct {
	Type        string `yaml:"type"`
	Value       string `yaml:"value"`
	Description string `yaml:"description"`
	Transport   string `yaml:"transport"`
	URL         string `yaml:"url"`
}

type frontmatter struct {
	Name                   string            `yaml:"name"`
	Description            string            `yaml:"description"`
	DisableModelInvocation bool              `yaml:"disable-model-invocation"`
	Codex                  *codexFrontmatter `yaml:"codex"`
}

func (fm frontmatter) codexConfig() *CodexConfig {
	if fm.Codex == nil {
		return nil
	}
	cfg := &CodexConfig{
		DisplayName:      fm.Codex.DisplayName,
		ShortDescription: fm.Codex.ShortDescription,
		IconSmall:        fm.Codex.IconSmall,
		IconLarge:        fm.Codex.IconLarge,
		BrandColor:       fm.Codex.BrandColor,
		DefaultPrompt:    fm.Codex.DefaultPrompt,
	}
	if fm.Codex.Dependencies != nil {
		for _, t := range fm.Codex.Dependencies.Tools {
			cfg.Dependencies = append(cfg.Dependencies, CodexToolDependency{
				Type:        t.Type,
				Value:       t.Value,
				Description: t.Description,
				Transport:   t.Transport,
				URL:         t.URL,
			})
		}
	}
	return cfg
}

func Scan(root string) ([]Skill, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("skillsrepo: read root %q: %w", root, err)
	}

	var skills []Skill
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skillMDPath := filepath.Join(root, entry.Name(), "SKILL.md")
		raw, err := os.ReadFile(skillMDPath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("skillsrepo: read %q: %w", skillMDPath, err)
		}

		fm, err := parseFrontmatter(raw)
		if err != nil {
			return nil, fmt.Errorf("skillsrepo: %q: %w", skillMDPath, err)
		}

		skills = append(skills, Skill{
			Name:                   fm.Name,
			Description:            fm.Description,
			Dir:                    entry.Name(),
			DisableModelInvocation: fm.DisableModelInvocation,
			Codex:                  fm.codexConfig(),
		})
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// pluginDirName はplugin機構向けの生成物を置くディレクトリ名。
const pluginDirName = ".claude-plugin"

// agentsDirName はCodex向けのagents/openai.yamlを置くディレクトリ名。
const agentsDirName = "agents"

// assetsDirName はagents/openai.yamlのicon_small/icon_largeが参照する画像を置くディレクトリ名。
const assetsDirName = "assets"

// allowedSkillDirEntries は各skillディレクトリ直下に許可するエントリ名。
// これ以外(hooks/・.mcp.json・commands/等)が見つかった場合はエラーにする。
// skill配下はそのままplugin(Claude Code/Codex)として配布されるため、
// hooksやMCPサーバ定義が紛れ込むと意図せず任意コマンド実行等が配布されてしまう。
var allowedSkillDirEntries = map[string]bool{
	"SKILL.md":    true,
	"README.md":   true,
	pluginDirName: true,
	agentsDirName: true,
	assetsDirName: true,
}

// allowedNestedDirEntries は、上記のうちディレクトリであるエントリそれぞれの
// 直下に許可するファイル名。assetsDirNameは可変のファイル名を許可するため、
// ここではなく拡張子ベースのallowedAssetExtensionsで別途チェックする。
var allowedNestedDirEntries = map[string]map[string]bool{
	pluginDirName: {"plugin.json": true},
	agentsDirName: {"openai.yaml": true},
}

// allowedAssetExtensions はassets/配下に置ける画像ファイルの拡張子(小文字)。
var allowedAssetExtensions = map[string]bool{
	".png":  true,
	".svg":  true,
	".jpg":  true,
	".jpeg": true,
	".gif":  true,
	".webp": true,
}

// CheckAllowedContents は各skillディレクトリの直下が、想定するファイル
// (SKILL.md / README.md / .claude-plugin/plugin.json / agents/openai.yaml /
// assets/配下の画像ファイル)だけで構成されていることを確認する。
func CheckAllowedContents(root string, skills []Skill) error {
	for _, s := range skills {
		dir := filepath.Join(root, s.Dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("skillsrepo: read skill dir %q: %w", dir, err)
		}
		for _, e := range entries {
			if !allowedSkillDirEntries[e.Name()] {
				return fmt.Errorf("skillsrepo: unexpected path %q in skill directory %q (only SKILL.md, README.md, %s/, %s/, %s/ are allowed)", filepath.Join(s.Dir, e.Name()), s.Dir, pluginDirName, agentsDirName, assetsDirName)
			}

			if e.Name() == assetsDirName {
				if !e.IsDir() {
					return fmt.Errorf("skillsrepo: %q must be a directory", filepath.Join(s.Dir, e.Name()))
				}
				if err := checkAssetsDir(dir, s.Dir); err != nil {
					return err
				}
				continue
			}

			allowedNested, ok := allowedNestedDirEntries[e.Name()]
			if !ok {
				continue
			}
			if !e.IsDir() {
				return fmt.Errorf("skillsrepo: %q must be a directory", filepath.Join(s.Dir, e.Name()))
			}
			nestedEntries, err := os.ReadDir(filepath.Join(dir, e.Name()))
			if err != nil {
				return fmt.Errorf("skillsrepo: read %q: %w", filepath.Join(s.Dir, e.Name()), err)
			}
			for _, ne := range nestedEntries {
				if !allowedNested[ne.Name()] {
					return fmt.Errorf("skillsrepo: unexpected path %q in %q (only %s allowed)", filepath.Join(s.Dir, e.Name(), ne.Name()), filepath.Join(s.Dir, e.Name()), strings.Join(sortedKeys(allowedNested), ", "))
				}
			}
		}
	}
	return nil
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func checkAssetsDir(skillDir, skillRelDir string) error {
	assetEntries, err := os.ReadDir(filepath.Join(skillDir, assetsDirName))
	if err != nil {
		return fmt.Errorf("skillsrepo: read %q: %w", filepath.Join(skillRelDir, assetsDirName), err)
	}
	for _, ae := range assetEntries {
		relPath := filepath.Join(skillRelDir, assetsDirName, ae.Name())
		if ae.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("skillsrepo: unexpected symlink %q in %q (only regular image files are allowed)", relPath, filepath.Join(skillRelDir, assetsDirName))
		}
		if ae.IsDir() {
			return fmt.Errorf("skillsrepo: unexpected directory %q in %q (only image files are allowed)", relPath, filepath.Join(skillRelDir, assetsDirName))
		}
		if !allowedAssetExtensions[strings.ToLower(filepath.Ext(ae.Name()))] {
			return fmt.Errorf("skillsrepo: unexpected file %q in %q (only image files are allowed)", relPath, filepath.Join(skillRelDir, assetsDirName))
		}
	}
	return nil
}

func parseFrontmatter(raw []byte) (frontmatter, error) {
	const delim = "---"
	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 || lines[0] != delim {
		return frontmatter{}, fmt.Errorf("missing frontmatter delimiter")
	}

	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if lines[i] == delim {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		return frontmatter{}, fmt.Errorf("missing closing frontmatter delimiter")
	}

	yamlBody := strings.Join(lines[1:closeIdx], "\n")

	var fm frontmatter
	if err := yaml.Unmarshal([]byte(yamlBody), &fm); err != nil {
		return frontmatter{}, fmt.Errorf("invalid frontmatter yaml: %w", err)
	}
	if fm.Name == "" {
		return frontmatter{}, fmt.Errorf("frontmatter name is empty")
	}
	if fm.Description == "" {
		return frontmatter{}, fmt.Errorf("frontmatter description is empty")
	}
	if fm.Codex != nil && fm.Codex.Dependencies != nil {
		for _, t := range fm.Codex.Dependencies.Tools {
			if t.Type == "" {
				return frontmatter{}, fmt.Errorf("codex.dependencies.tools[].type is empty")
			}
			if t.Value == "" {
				return frontmatter{}, fmt.Errorf("codex.dependencies.tools[].value is empty")
			}
		}
	}
	return fm, nil
}
