# Dagim Repository Guidance

## Product invariants

- `dagim FILE` opens the TUI; keep that as the default invocation.
- The on-disk format is `# dagim v1`. Do not couple its version to the CLI
  JSON schema version.
- Scriptable JSON is a public interface. Preserve the compatibility rules in
  `docs/cli-json.md` and update contract tests when it changes.
- Mutations must preserve DAG and completion-state validity and save atomically.

## Working from a checkout

- Installation is unnecessary: run commands with `go run ./cmd/dagim ...`.
- Prefer `--json` when Dagim is being used as a tool by a script or agent.
- When evaluating the CLI as a user would, interact through the CLI instead of
  parsing or editing the `.dagim` file directly.
- `bin/` contains ignored local builds. `personal/` contains ignored user data;
  never add it to commits.
- Preserve unrelated work in the checkout and keep behavior changes tested and
  documented.

## Validation and releases

- For ordinary changes, run `go test ./...` and `go vet ./...`.
- After completing a feature and before handing it off for commit or deployment,
  run `go build -o bin/dagim ./cmd/dagim` so `./bin/dagim` is ready for local
  user testing and reflects the current checkout.
- Before a release, follow `docs/releasing.md` and run
  `scripts/release-check vX.Y.Z` from a clean release commit.
- GitHub releases are source releases. Homebrew builds the tag from source via
  `tunesmith/homebrew-tap/Formula/dagim.rb`.
