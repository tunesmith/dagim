# Releasing Dagim

Dagim uses semantic versions, annotated Git tags, GitHub source releases, and a
source-building formula in
[`tunesmith/homebrew-tap`](https://github.com/tunesmith/homebrew-tap).

The application repository and Homebrew tap are released separately. Do not
publish a tag until the application pull request is merged and all checks pass.

## 1. Prepare the release

Start from an up-to-date `main` and create a focused branch. Choose the version
before changing release metadata: patch releases contain compatible fixes;
minor releases add compatible functionality.

Finalize `CHANGELOG.md` with a dated heading in this form:

```markdown
## 1.2.0 (2026-08-01)
```

Update user documentation and help text for user-visible changes. Commit the
complete release candidate, then run the non-publishing preflight from its clean
commit:

```sh
scripts/release-check v1.2.0
```

The preflight verifies the changelog, local and remote tag availability, clean
worktree, tests, race tests, vet, whitespace, and release-style version
injection. It does not modify Git, GitHub, Homebrew, or the working tree.

## 2. Merge the release pull request

Push the branch, open a ready pull request, and wait for GitHub Actions. Use a
squash merge so `main` receives one cohesive public commit while the pull
request retains the development history and discussion.

After the merge, fast-forward the local `main` and verify that it is clean and
matches `origin/main`:

```sh
git switch main
git pull --ff-only origin main
git status -sb
```

GitHub automatically deletes merged head branches. Prune that remote-tracking
reference and delete the local branch:

```sh
git fetch --prune
git branch -D <release-branch>
```

The capital `-D` is expected after a squash merge: the combined contents are on
`main`, but the feature branch's individual commits are not ancestors of the
squash commit. Verify the pull request and `main` before deleting it.

## 3. Tag and publish on GitHub

Create the annotated tag at the verified `main` commit and push only that tag:

```sh
git tag -a v1.2.0 -m "dagim v1.2.0"
git push origin v1.2.0
```

Pushing `v*` triggers `.github/workflows/release.yml`, which independently runs
tests, vet, and version-injection verification before creating the GitHub
release. Wait for the workflow and verify that the tag peels to the same commit
as `main`:

```sh
gh run list --workflow release.yml --limit 5
gh release view v1.2.0
git rev-list -n 1 v1.2.0
```

GitHub releases are source-only by design. Replace sparse generated notes with
the corresponding changelog section and retain a full-changelog comparison
link. Do not add binary assets unless the distribution model deliberately
changes.

## 4. Publish the Homebrew formula

Download the public tagged archive and calculate the checksum from that exact
artifact:

```sh
curl -fL \
  https://github.com/tunesmith/dagim/archive/refs/tags/v1.2.0.tar.gz \
  -o /tmp/dagim-v1.2.0.tar.gz
shasum -a 256 /tmp/dagim-v1.2.0.tar.gz
```

In `tunesmith/homebrew-tap`, update `Formula/dagim.rb` with the new tag URL and
SHA-256. Keep its test block exercising `--version`, compatibility checking,
JSON reads, and a mutation. Run `brew style` before committing the formula,
then commit and push the tap's `main` branch.

Homebrew 6 audits named formulae rather than filesystem paths, so refresh the
published tap before running the strict audit:

```sh
brew update
brew audit --strict --online tunesmith/tap/dagim
```

## 5. Verify the public installation

Upgrade and test the formula through Homebrew:

```sh
brew upgrade tunesmith/tap/dagim
brew test tunesmith/tap/dagim
```

Do not assume the unqualified `dagim` command resolves to Homebrew; Go installs
or development builds may appear earlier in `PATH`. Test the installed formula
explicitly:

```sh
dagim_brew_bin="$(brew --prefix dagim)/bin/dagim"
"$dagim_brew_bin" --version
"$dagim_brew_bin" check examples/gumbo.dagim
"$dagim_brew_bin" ready examples/gumbo.dagim --json
```

Finish by confirming that the application `main`, annotated tag, GitHub
release, tap commit, formula URL and checksum, and installed version all agree.
