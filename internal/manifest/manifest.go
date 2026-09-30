package manifest

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/sgash708/skillctl/internal/skillsrepo"
	"gopkg.in/yaml.v3"
)

const PluginVersion = "0.0.0"

// Meta represents the marketplace name and owner written to marketplace.json.
type Meta struct {
	Name  string
	Owner string
}

var jsonMarshalIndent = json.MarshalIndent
var yamlMarshal = yaml.Marshal

type Plugin struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

type Owner struct {
	Name string `json:"name"`
}

type Marketplace struct {
	Name    string   `json:"name"`
	Owner   Owner    `json:"owner"`
	Plugins []Plugin `json:"plugins"`
}

type PluginManifest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// OpenAIYAML represents the contents of agents/openai.yaml for Codex.
// Field spec: https://developers.openai.com/codex/skills (agents/openai.yaml)
type OpenAIYAML struct {
	Interface    *OpenAIInterface    `yaml:"interface,omitempty"`
	Dependencies *OpenAIDependencies `yaml:"dependencies,omitempty"`
	Policy       OpenAIPolicy        `yaml:"policy"`
}

type OpenAIInterface struct {
	DisplayName      string `yaml:"display_name,omitempty"`
	ShortDescription string `yaml:"short_description,omitempty"`
	IconSmall        string `yaml:"icon_small,omitempty"`
	IconLarge        string `yaml:"icon_large,omitempty"`
	BrandColor       string `yaml:"brand_color,omitempty"`
	DefaultPrompt    string `yaml:"default_prompt,omitempty"`
}

type OpenAIDependencies struct {
	Tools []OpenAITool `yaml:"tools,omitempty"`
}

type OpenAITool struct {
	Type        string `yaml:"type"`
	Value       string `yaml:"value"`
	Description string `yaml:"description,omitempty"`
	Transport   string `yaml:"transport,omitempty"`
	URL         string `yaml:"url,omitempty"`
}

type OpenAIPolicy struct {
	AllowImplicitInvocation bool `yaml:"allow_implicit_invocation"`
}

// BuildOpenAIYAML builds the contents of agents/openai.yaml from a skill's frontmatter.
// A skill that specifies neither a codex section nor disable-model-invocation is treated as not to be generated (ok=false),
// so as not to mass-produce noise files containing only default values.
func BuildOpenAIYAML(s skillsrepo.Skill) (OpenAIYAML, bool) {
	if s.Codex == nil && !s.DisableModelInvocation {
		return OpenAIYAML{}, false
	}

	y := OpenAIYAML{
		Policy: OpenAIPolicy{AllowImplicitInvocation: !s.DisableModelInvocation},
	}

	if c := s.Codex; c != nil {
		iface := OpenAIInterface{
			DisplayName:      c.DisplayName,
			ShortDescription: c.ShortDescription,
			IconSmall:        c.IconSmall,
			IconLarge:        c.IconLarge,
			BrandColor:       c.BrandColor,
			DefaultPrompt:    c.DefaultPrompt,
		}
		if iface != (OpenAIInterface{}) {
			y.Interface = &iface
		}
		if len(c.Dependencies) > 0 {
			deps := &OpenAIDependencies{}
			for _, t := range c.Dependencies {
				deps.Tools = append(deps.Tools, OpenAITool{
					Type:        t.Type,
					Value:       t.Value,
					Description: t.Description,
					Transport:   t.Transport,
					URL:         t.URL,
				})
			}
			y.Dependencies = deps
		}
	}

	return y, true
}

func BuildMarketplace(meta Meta, skills []skillsrepo.Skill) Marketplace {
	plugins := make([]Plugin, 0, len(skills))
	for _, s := range skills {
		plugins = append(plugins, Plugin{
			Name:        s.Name,
			Source:      "./" + s.Dir,
			Description: s.Description,
			Version:     PluginVersion,
		})
	}
	return Marketplace{
		Name:    meta.Name,
		Owner:   Owner{Name: meta.Owner},
		Plugins: plugins,
	}
}

func BuildPluginManifest(s skillsrepo.Skill) PluginManifest {
	return PluginManifest{Name: s.Name, Description: s.Description, Version: PluginVersion}
}

func marketplacePath(root string) string {
	return filepath.Join(root, ".claude-plugin", "marketplace.json")
}

func pluginPath(root string, s skillsrepo.Skill) string {
	return filepath.Join(root, s.Dir, ".claude-plugin", "plugin.json")
}

func openAIYAMLPath(root string, s skillsrepo.Skill) string {
	return filepath.Join(root, s.Dir, "agents", "openai.yaml")
}

func Write(root string, meta Meta, skills []skillsrepo.Skill) ([]string, error) {
	var written []string

	mp := BuildMarketplace(meta, skills)
	mpBytes, err := jsonMarshalIndent(mp, "", "  ")
	if err != nil {
		return nil, err
	}
	mpPath := marketplacePath(root)
	if err := os.MkdirAll(filepath.Dir(mpPath), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(mpPath, append(mpBytes, '\n'), 0o644); err != nil {
		return nil, err
	}
	written = append(written, ".claude-plugin/marketplace.json")

	for _, s := range skills {
		pm := BuildPluginManifest(s)
		pmBytes, err := jsonMarshalIndent(pm, "", "  ")
		if err != nil {
			return nil, err
		}
		p := pluginPath(root, s)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(p, append(pmBytes, '\n'), 0o644); err != nil {
			return nil, err
		}
		written = append(written, filepath.Join(s.Dir, ".claude-plugin", "plugin.json"))

		oaiPath := openAIYAMLPath(root, s)
		if y, ok := BuildOpenAIYAML(s); ok {
			yBytes, err := yamlMarshal(y)
			if err != nil {
				return nil, err
			}
			if err := os.MkdirAll(filepath.Dir(oaiPath), 0o755); err != nil {
				return nil, err
			}
			if err := os.WriteFile(oaiPath, yBytes, 0o644); err != nil {
				return nil, err
			}
			written = append(written, filepath.Join(s.Dir, "agents", "openai.yaml"))
		} else if err := os.Remove(oaiPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	return written, nil
}

func Check(root string, meta Meta, skills []skillsrepo.Skill) ([]string, error) {
	var stale []string

	wantMP := BuildMarketplace(meta, skills)
	wantMPBytes, err := jsonMarshalIndent(wantMP, "", "  ")
	if err != nil {
		return nil, err
	}
	gotMPBytes, err := os.ReadFile(marketplacePath(root))
	switch {
	case errors.Is(err, fs.ErrNotExist):
		stale = append(stale, ".claude-plugin/marketplace.json")
	case err != nil:
		return nil, err
	case string(gotMPBytes) != string(append(wantMPBytes, '\n')):
		stale = append(stale, ".claude-plugin/marketplace.json")
	}

	for _, s := range skills {
		wantPM := BuildPluginManifest(s)
		wantPMBytes, err := jsonMarshalIndent(wantPM, "", "  ")
		if err != nil {
			return nil, err
		}
		gotPMBytes, err := os.ReadFile(pluginPath(root, s))
		switch {
		case errors.Is(err, fs.ErrNotExist):
			stale = append(stale, filepath.Join(s.Dir, ".claude-plugin", "plugin.json"))
		case err != nil:
			return nil, err
		case string(gotPMBytes) != string(append(wantPMBytes, '\n')):
			stale = append(stale, filepath.Join(s.Dir, ".claude-plugin", "plugin.json"))
		}

		oaiPath := openAIYAMLPath(root, s)
		wantY, wantOK := BuildOpenAIYAML(s)
		gotYBytes, err := os.ReadFile(oaiPath)
		switch {
		case !wantOK && errors.Is(err, fs.ErrNotExist):
			// Not a generation target and no actual file exists: up to date
		case !wantOK && err == nil:
			// The codex section etc. was removed but the generated file remains: stale
			stale = append(stale, filepath.Join(s.Dir, "agents", "openai.yaml"))
		case !wantOK:
			return nil, err
		case errors.Is(err, fs.ErrNotExist):
			stale = append(stale, filepath.Join(s.Dir, "agents", "openai.yaml"))
		case err != nil:
			return nil, err
		default:
			wantYBytes, err := yamlMarshal(wantY)
			if err != nil {
				return nil, err
			}
			if string(gotYBytes) != string(wantYBytes) {
				stale = append(stale, filepath.Join(s.Dir, "agents", "openai.yaml"))
			}
		}
	}

	return stale, nil
}
