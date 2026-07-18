# Contributing

Thanks for considering a contribution to dagim.

By submitting a contribution, you agree that your contribution is licensed under
the same license as this project: GPL-3.0-or-later. You also confirm that you
have the right to submit the contribution under those terms.

Before opening a pull request, please run:

```sh
go test ./...
go vet ./...
```

Keep changes focused and include tests for behavior changes when practical.

Repository-specific guidance for coding agents is in `AGENTS.md`. For a public
release, follow `docs/releasing.md` and run the complete non-publishing
preflight documented there.
