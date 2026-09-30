# skillctl

[日本語](README.ja.md)

A CLI that imports skills from a GitHub repository into **Claude Code** and **Codex** as plugins, and generates the `marketplace.json` / `plugin.json` files that make such a repository installable.

- `skillctl import` installs skills from a skill repository into Claude Code, Codex, or both, with an interactive picker or from arguments.
- `skillctl generate` builds `.claude-plugin/marketplace.json`, each skill's `plugin.json` and, optionally, Codex's `agents/openai.yaml` from your `SKILL.md` files. `--check` fails when they are out of date, which suits CI.

## Install

Download the archive for your OS and architecture from [Releases](https://github.com/sgash708/skillctl/releases) (`skillctl_<os>_<arch>.tar.gz`, `.zip` on Windows), extract it, and put `skillctl` on your `PATH`. With Go installed:

```bash
go install github.com/sgash708/skillctl/cmd/skillctl@latest
```

Run `skillctl --version` to check the installation. Windows binaries are built but not regularly tested.

## Import skills

```bash
# Interactive: pick skills from a list, then choose where to import them
skillctl import --repo owner/skills

# Non-interactive
skillctl import <skill>... --repo owner/skills --target claude|codex|both --yes
```

| Flag | Description |
| --- | --- |
| `--repo` | GitHub repository that hosts the skills, as `owner/name`. Can also be set with the `SKILLCTL_REPO` environment variable. |
| `--target` | `claude`, `codex`, or `both` (default). |
| `--marketplace` | Marketplace name. Must match `generate --name`. Defaults to the repository name. |
| `--source` | Marketplace source. Defaults to `https://github.com/<repo>`. A local directory path also works, which is handy while developing a skill repository. |
| `-y`, `--yes` | Non-interactive mode. Requires at least one skill name. |

Requirements: the `claude` and/or `codex` CLI on `PATH`, matching `--target`. Interactive mode also needs the [GitHub CLI](https://cli.github.com/) (`gh`), logged in if the repository is private.

If `https://github.com/...` is unreachable, for example behind a proxy, pass an SSH URL instead:

```bash
skillctl import <skill> --repo owner/skills --yes --source git@github.com:owner/skills.git
```

> **Security:** a skill is a plugin that runs inside your agent, and can include hooks, scripts and MCP servers. Import only from repositories you trust, and read a skill before you install it.

## Generate manifests

Run this in the root of a skill repository:

```bash
skillctl generate --owner <owner> --name <marketplace-name> [<root>]
```

`--owner` is required. `--name` defaults to the name of the root directory.

Expected layout: every direct subdirectory of the root that contains a `SKILL.md` is one skill. Hidden directories are ignored.

```text
my-skills/
├── .claude-plugin/marketplace.json   # generated
├── code-review/
│   ├── SKILL.md
│   └── .claude-plugin/plugin.json    # generated
└── release-notes/
    ├── SKILL.md
    ├── scripts/                      # any other files are fine
    └── agents/openai.yaml            # generated, only if you ask for it (see below)
```

`SKILL.md` needs a frontmatter with `name` and `description`:

```markdown
---
name: code-review
description: Review a diff for correctness bugs
---
```

To generate Codex's `agents/openai.yaml`, add `disable-model-invocation: true` or a `codex:` section to the frontmatter. The `codex:` section accepts `display_name`, `short_description`, `icon_small`, `icon_large`, `brand_color`, `default_prompt` and `dependencies.tools` (see the [Codex skills docs](https://developers.openai.com/codex/skills)). `icon_small` and `icon_large` are paths relative to the skill directory, such as `./assets/icon.png`.

Options:

- `--check` writes nothing and exits non-zero when generated files are stale. Run it in CI.
- `--strict` rejects skill directories that hold anything besides `SKILL.md`, `README.md`, `.claude-plugin/`, `agents/` and `assets/` (images only, no subdirectories). Use it if you want a skill repository to stay free of hooks, MCP definitions and scripts.

## Development

```bash
go test ./... -coverprofile=coverage.out
```

## License

[MIT](LICENSE)
