# Contributing

Issues and pull requests are welcome.

- Run `go test ./... -coverprofile=coverage.out` before opening a pull request. CI requires 100% coverage, except for the thin wiring files listed in `.octocov.yml`.
- Format with `gofmt`.
- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`, `fix:`, `docs:`, ...).
- For a larger change, open an issue first so we can agree on the direction.
