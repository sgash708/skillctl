# skillctl

GitHubで管理しているskillリポジトリを、Claude Code / Codexへ`plugin`としてimportするCLI。
skillリポジトリ側の`marketplace.json`・`plugin.json`を`SKILL.md`から生成する`generate`も持つ。

## インストール

GitHub Releasesから、自分のOS/archに合ったバイナリをダウンロードする。ファイル名は
`skillctl_<os>_<arch>.tar.gz`(例: `skillctl_darwin_arm64.tar.gz`)。展開して`skillctl`を
PATHの通った場所に置く。Goがあれば次でも入る。

```bash
go install github.com/sgash708/skillctl/cmd/skillctl@latest
```

## import

```bash
# 対話モード(skill名を省略): 一覧からskillを選び、import先も選ぶ
skillctl import --repo owner/skills

# 非対話モード: skill名を指定する
skillctl import <skill> --repo owner/skills --target claude|codex|both --yes
```

| フラグ | 説明 |
| --- | --- |
| `--repo` | skillを管理しているGitHubリポジトリ(`owner/name`)。環境変数`SKILLCTL_REPO`でも指定できる |
| `--target` | `claude`・`codex`・`both`(デフォルト) |
| `--marketplace` | marketplace名。`generate --name`と揃える。省略時はリポジトリ名 |
| `--source` | marketplaceのsource。省略時は`https://github.com/<repo>`。ローカルディレクトリのパスも指定できる |
| `-y`, `--yes` | 非対話モード。skill名が最低1つ必要 |

対話モードでは`gh`(skill一覧の取得用)が、importする対象に応じて`claude`/`codex`がPATHに必要。

プロキシ環境などで`https://github.com/...`に繋がらない場合は、SSHのURLを`--source`に渡すと通ることがある。

```bash
skillctl import <skill> --repo owner/skills --yes --source git@github.com:owner/skills.git
```

## generate

skillリポジトリのルートで、各skillの`SKILL.md`から`.claude-plugin/marketplace.json`と
`<skill>/.claude-plugin/plugin.json`を生成する。

```bash
skillctl generate --owner owner --name skills [<root>]
```

- `--owner`は必須。`--name`は省略するとルートのディレクトリ名になる
- `--check`を付けると書き込まず、生成物が古いときに非0で終了する。CIで差分検出に使える
- `SKILL.md`のfrontmatterに`disable-model-invocation: true`または`codex:`セクション
  (`display_name`/`short_description`/`icon_small`/`icon_large`/`brand_color`/
  `default_prompt`/`dependencies.tools`)を書くと、
  [Codexのskillメタデータ](https://developers.openai.com/codex/skills)である
  `<skill>/agents/openai.yaml`も生成する。どちらも無ければ生成しない
- `icon_small`/`icon_large`はskillディレクトリからの相対パス(`./assets/xxx.png`等)で書く。
  画像は`<skill>/assets/`直下に置く。`assets/`には画像(`.png`/`.svg`/`.jpg`/`.jpeg`/`.gif`/`.webp`)だけ置け、サブディレクトリは作れない

## 開発

```bash
go test ./... -coverprofile=coverage.out
```

## ライセンス

MIT
