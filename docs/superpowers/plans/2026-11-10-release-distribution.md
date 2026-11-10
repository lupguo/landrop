# Release Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Pushing a `v*` tag builds signed and notarized release archives, publishes a Homebrew cask, and lets users install LanDrop with one command on mac, Linux and Windows. The default port becomes 9958.

**Architecture:** A GitHub Actions workflow runs on tag pushes. It tests, builds the web UI, then runs GoReleaser. GoReleaser cross-compiles 5 targets, signs and notarizes the darwin binaries cross-platform, creates versionless archives plus `checksums.txt`, and pushes `Casks/landrop.rb` to `lupguo/homebrew-tap`. Two install scripts download from `releases/latest/download/` and verify checksums.

**Tech Stack:** GitHub Actions, GoReleaser OSS v2 (>= 2.19), POSIX sh, PowerShell 5.1+, Go 1.26, Node 22.

**Spec:** `docs/superpowers/specs/2026-11-10-release-distribution-design.md`

## Global Constraints

- Release trigger: `push: tags: ['v*']` only. Tags containing `-` are prereleases: they are not "latest" and do not update the tap.
- Targets: darwin/arm64, darwin/amd64, windows/amd64, linux/amd64, linux/arm64. `CGO_ENABLED=0`, `-trimpath`, `-s -w`.
- ldflags: `-X github.com/lupguo/landrop/internal/version.Version={{.Tag}}` (with the `v` prefix), plus `.Commit` and `.Date`.
- Archive names have no version: `landrop_<os>_<arch>.tar.gz`, or `.zip` for Windows. Checksums file: `checksums.txt` (SHA-256).
- Secret names, exactly: `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `MACOS_NOTARY_ISSUER_ID`, `MACOS_NOTARY_KEY_ID`, `MACOS_NOTARY_KEY`, `HOMEBREW_TAP_GITHUB_TOKEN`. They live in GitHub Environment `release`.
- Never strip `com.apple.quarantine`. Never sudo automatically in scripts.
- Install URLs: `https://raw.githubusercontent.com/lupguo/landrop/main/scripts/install.sh` and `.../scripts/install.ps1`. Brew: `brew install lupguo/tap/landrop`.
- Default port 9958. The e2e port 18848 stays unchanged.
- Docs that users read (README, setup manual) are bilingual where noted. `docs/release-setup.md` is Chinese.

## Review Focus

1. Checksum mismatch or a missing entry in `checksums.txt`: the install script must exit non-zero and install nothing. Test: Task 3, `tampered checksum` case.
2. Unsupported OS/arch (e.g. `uname -m` = `i686`, `uname -s` = `FreeBSD`): exit with a clear message naming the platform. Test: Task 3, `unsupported platform` case, using `LANDROP_UNAME_S` / `LANDROP_UNAME_M` overrides.
3. `/usr/local/bin` not writable and no `LANDROP_INSTALL_DIR`: install to `~/.local/bin` and print a PATH hint. Never prompt for sudo. Test: Task 3, `fallback dir` case, with `HOME` set to a temp dir and the default dir overridden to a read-only temp dir via `LANDROP_DEFAULT_DIR`.
4. Re-running the installer over an existing install (upgrade) overwrites cleanly. Test: Task 3, run the install twice into the same dir and expect success both times.
5. A release run without secrets (a fork, or before setup) must skip signing instead of failing. Test: Task 2, `goreleaser release --snapshot --clean` with no `MACOS_*` env set succeeds.
6. The Windows installer must not duplicate its PATH entry when run again. Covered by Task 4, step 3 logic, and a manual check documented there.

---

### Task 0: Branch and commit the spec

- [ ] **Step 1:** HEAD is currently detached, and `imgs/cmd.jpg` and `imgs/web.jpg` are staged. Run `git switch -c release-distribution`. Leave the images staged; Task 7 commits them.
- [ ] **Step 2:** Run `git commit -m "docs: release distribution design and plan" -- docs/superpowers/specs/2026-11-10-release-distribution-design.md docs/superpowers/plans/2026-11-10-release-distribution.md`. Expected: a commit containing only those two files (`git show --stat HEAD`).

### Task 1: Default port 9958 everywhere

**Files:**
- Modify: `internal/config/config.go:89` and every file listed by `git grep -lP '(?<!\d)8848(?!\d)'`, except the release-distribution spec and this plan. That covers the Go tests, `internal/server/server.go`, `web/vite.config.ts`, `README.md`, `LandropPRD.md`, `docs/acceptance-v0.1.md`, and the earlier specs/plans.

- [ ] **Step 1: Change the test first.** In `internal/config/config_test.go:15`, set the expected value to `c.Port != 9958`. At line 78, set the expected help string to `"9958"`.
- [ ] **Step 2:** Run `go test ./internal/config/`. Expected: FAIL (the default is still 8848).
- [ ] **Step 3:** Replace the standalone `8848` with `9958` in the files above: `git grep -lP '(?<!\d)8848(?!\d)' | grep -v release-distribution | xargs perl -pi -e 's/(?<!\d)8848(?!\d)/9958/g'`.
- [ ] **Step 4:** Edit `LandropPRD.md` Q1 so it reads as decided: `| Q1 | 默认端口 | 已定 9958 |`. Edit the MVP spec's Q1 row (`保持 9958 [我的判断]` after the replace) to `9958 [用户定 2026-11-10]`.
- [ ] **Step 5: Verify.**
  - `git grep -nP '(?<!\d)8848(?!\d)' | grep -v release-distribution` → no output.
  - `git grep -n 18848` still shows Makefile and web e2e.
  - `go test ./...` → PASS.
  - `cd web && npm test` → PASS.
- [ ] **Step 6:** Run `git commit -am "feat: default port 9958"`. Before running it, confirm with `git status` that no images get swept in. `-a` only stages tracked modifications, and the images are already staged; if they would be included, use `git commit -- <files>` instead.

### Task 2: GoReleaser config

**Files:**
- Create: `.goreleaser.yaml`
- Modify: `Makefile` (add a `release-snapshot` target)

**Interfaces:**
- Produces:
  - Build ids `darwin` (goos darwin) and `others` (linux, windows).
  - Artifacts in `dist/`: `landrop_darwin_arm64.tar.gz`, `landrop_darwin_amd64.tar.gz`, `landrop_linux_amd64.tar.gz`, `landrop_linux_arm64.tar.gz`, `landrop_windows_amd64.zip`, `checksums.txt`.
  - Each archive contains `landrop` (`landrop.exe` on Windows) and `README.md` at the archive root, with no wrapping directory.

- [ ] **Step 1:** Install the local tooling: `brew install goreleaser shellcheck actionlint`. Check with `goreleaser --version` → >= 2.19.
- [ ] **Step 2:** Write `.goreleaser.yaml` (`version: 2`) with these settings:
  - `builds`: two entries, `darwin` (amd64, arm64) and `others` (linux amd64/arm64, windows amd64). Use `main: ./cmd/landrop`, `binary: landrop`, `env: [CGO_ENABLED=0]`, `flags: [-trimpath]`, and the ldflags from Global Constraints. No `before.hooks`: the web build happens in CI or Make beforehand.
  - `archives`: `name_template: "landrop_{{ .Os }}_{{ .Arch }}"`, `formats: [tar.gz]`, a windows override to `[zip]`, `files: [README.md]`, `wrap_in_directory: false`.
  - `checksum.name_template: checksums.txt`.
  - `notarize.macos`: one entry with `enabled: '{{ isEnvSet "MACOS_SIGN_P12" }}'`, `ids: [darwin]`. Under `sign`: certificate and password from env. Under `notarize`: issuer_id, key_id, key from env, `wait: true`, `timeout: 20m`.
  - `release.prerelease: auto`.
  - `homebrew_casks`: `name: landrop`, `binaries: [landrop]`, `directory: Casks`, `homepage: https://github.com/lupguo/landrop`, and a one-line description. Set `repository` to owner `lupguo`, name `homebrew-tap`, token `{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}`. Set `skip_upload: auto`. **No hooks.**
  - `changelog`: use the default, sorted ascending, excluding `^docs:` and `^test:`.
- [ ] **Step 3:** Add a `release-snapshot: web` target to `Makefile` that runs `goreleaser release --snapshot --clean`. Add it to `.PHONY`.
- [ ] **Step 4: Verify.**
  - `goreleaser check` → no errors and no deprecation warnings.
  - `env -u MACOS_SIGN_P12 make release-snapshot` → succeeds, and the log shows notarize skipped (Review Focus 5).
  - `ls dist/*.tar.gz dist/*.zip dist/checksums.txt` lists exactly the 6 files from Interfaces.
  - `tar -tzf dist/landrop_darwin_arm64.tar.gz` → `landrop` and `README.md`.
  - `tar -xzf` the arm64 archive into a temp dir, then run `./landrop --version` → `landrop v…-SNAPSHOT…`.
  - `cd dist && shasum -a 256 -c checksums.txt --ignore-missing` → all OK.
  - Inspect `dist/homebrew/Casks/landrop.rb`: it has no `xattr`, and its URLs use `releases/download/<tag>/landrop_darwin_*.tar.gz`.
- [ ] **Step 5:** Run `git add .goreleaser.yaml Makefile && git commit -m "build: goreleaser config with signing, notarization and homebrew cask"`.

### Task 3: `scripts/install.sh` with a test harness

**Files:**
- Create: `scripts/install.sh`, `scripts/test-install.sh`

**Interfaces:**
- Consumes: the artifact names from Task 2.
- Environment variables:
  - `LANDROP_VERSION`: a tag; default latest.
  - `LANDROP_INSTALL_DIR`
  - `LANDROP_DOWNLOAD_BASE`: test override. Default `https://github.com/lupguo/landrop/releases/latest/download`, or `.../releases/download/$LANDROP_VERSION` when a version is set.
  - `LANDROP_DEFAULT_DIR`: test override, default `/usr/local/bin`.
  - `LANDROP_UNAME_S`, `LANDROP_UNAME_M`: test overrides for `uname`.

- [ ] **Step 1: Write the harness first.** `scripts/test-install.sh` (bash) expects `dist/` from `make release-snapshot`. It serves `dist/` with `python3 -m http.server` on a random free port, sets `LANDROP_DOWNLOAD_BASE=http://127.0.0.1:<port>`, and runs these cases, each with a temp `HOME` and a temp install dir:
  - `happy path`: exit 0, `"$DIR/landrop" --version` prints `landrop v`.
  - `reinstall`: run again into the same dir → exit 0.
  - `tampered checksum`: serve a copy of `dist/` where `checksums.txt` has a wrong hash for this platform → non-zero exit, and `$DIR/landrop` does not exist.
  - `unsupported platform`: `LANDROP_UNAME_M=i686` → non-zero exit, stderr contains `i686`. `LANDROP_UNAME_S=FreeBSD` → non-zero exit, stderr contains `FreeBSD`.
  - `fallback dir`: no `LANDROP_INSTALL_DIR`, `LANDROP_DEFAULT_DIR` set to a `chmod 555` temp dir → installs to `$HOME/.local/bin/landrop`, and stdout mentions `PATH`.

  The harness prints `PASS <case>` / `FAIL <case>`, exits non-zero on any failure, and kills the server on exit.
- [ ] **Step 2:** Run `scripts/test-install.sh`. Expected: FAIL, because install.sh does not exist yet.
- [ ] **Step 3:** Write `scripts/install.sh` as POSIX sh with `set -eu`:
  - Map the platform: `Darwin`→darwin, `Linux`→linux; `x86_64|amd64`→amd64, `arm64|aarch64`→arm64. Anything else: error naming the value and pointing to the Releases page for manual download.
  - Download with curl (`-fsSL`), falling back to wget. Work in a `mktemp -d` directory, removed by a `trap` on exit.
  - Look up the archive's line in `checksums.txt`; a missing line is a failure. Compare against `sha256sum` or `shasum -a 256`; neither tool available is also a failure.
  - Extract the archive, `chmod +x`, then move into place atomically: copy to `$dir/.landrop.tmp`, then `mv`.
  - Choose the directory: `LANDROP_INSTALL_DIR`, else `LANDROP_DEFAULT_DIR` if it is writable, else `$HOME/.local/bin` (create it). If the chosen dir is not on `PATH`, print the `export PATH=...` line.
  - Finish by printing the `--version` output and `Run: landrop`.
- [ ] **Step 4: Verify.**
  - `shellcheck -s sh scripts/install.sh` → clean.
  - `make release-snapshot && scripts/test-install.sh` → all PASS.
  - Also run the harness inside Linux: `docker run --rm -v "$PWD":/w -w /w debian:stable-slim sh -c 'apt-get update -qq && apt-get install -y -qq curl python3 >/dev/null && bash scripts/test-install.sh'`. If Docker is unavailable, say so in the report instead of claiming Linux passed.
- [ ] **Step 5:** Run `git add scripts/install.sh scripts/test-install.sh && git commit -m "feat: install.sh for macOS and Linux with checksum verification"`.

### Task 4: `scripts/install.ps1`

**Files:**
- Create: `scripts/install.ps1`

- [ ] **Step 1:** Write it for Windows PowerShell 5.1:
  - Start with `$ErrorActionPreference = 'Stop'` and set TLS 1.2 explicitly via `[Net.ServicePointManager]::SecurityProtocol`.
  - Read the same `LANDROP_VERSION` and `LANDROP_DOWNLOAD_BASE` environment variables as install.sh.
  - Refuse to run on anything other than AMD64, checked via `$env:PROCESSOR_ARCHITECTURE`, with a clear message.
  - Download `landrop_windows_amd64.zip` and `checksums.txt` to a temp dir.
  - Compare `Get-FileHash -Algorithm SHA256` case-insensitively; on mismatch, throw without installing.
  - `Expand-Archive` into `$env:LOCALAPPDATA\Programs\landrop`, overwriting.
  - Add that dir to the User PATH via `[Environment]::SetEnvironmentVariable(..., 'User')` only if it is not already present (compare case-insensitively, trimming trailing `\`). Also update `$env:Path` for the current session.
  - Print the `landrop.exe --version` output and "open a new terminal, then run: landrop".
- [ ] **Step 2: Verify.**
  - If `pwsh` is available (`brew install --cask powershell` is optional), check that it parses: `pwsh -NoProfile -Command '$null = [scriptblock]::Create((Get-Content -Raw scripts/install.ps1))'` → no error.
  - Either way, record in the commit message body that a real Windows run is pending in the manual's first-release checklist (Task 6).
- [ ] **Step 3:** Run `git add scripts/install.ps1 && git commit -m "feat: install.ps1 for Windows with checksum verification"`.

### Task 5: Release workflow

**Files:**
- Create: `.github/workflows/release.yml`

- [ ] **Step 1:** Write the workflow:
  - Name `release`, trigger `on: push: tags: ['v*']`, top-level `permissions: contents: write`.
  - One job `goreleaser` on `ubuntu-latest` with `environment: release`, and `concurrency: release-${{ github.ref }}`.
  - Steps:
    1. `actions/checkout` with `fetch-depth: 0`.
    2. `actions/setup-go` with `go-version-file: go.mod`.
    3. `actions/setup-node` with node 22, npm cache on `web/package-lock.json`.
    4. `go test ./...`.
    5. `cd web && npm ci && npm test && npm run build`.
    6. `goreleaser/goreleaser-action` with `distribution: goreleaser`, `version: '~> v2'`, `args: release --clean`. Pass env: `GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}` plus the 6 secrets by their exact names.
  - Pin every third-party action to a full commit SHA, with the version tag in a trailing comment. Look up the SHAs with `gh api repos/<owner>/<repo>/git/ref/tags/<tag>`, dereferencing annotated tags.
- [ ] **Step 2: Verify.** Run `actionlint .github/workflows/release.yml` → clean. Then `grep -E 'uses: [^@]+@[0-9a-f]{40}' -c` on the file equals the number of `uses:` lines.
- [ ] **Step 3:** Run `git add .github/workflows/release.yml && git commit -m "ci: release workflow on version tags"`.

### Task 6: Setup manual `docs/release-setup.md` (Chinese)

**Files:**
- Create: `docs/release-setup.md`

- [ ] **Step 1:** Write chapters 0–10 exactly as listed in spec §4.7, using the secret table in spec §5 and the secret names from Global Constraints. Requirements per chapter:
  - Every step says where to click, what you get, and where to put it. Include exact commands: `base64 -i cert.p12 | pbcopy`, `base64 -i AuthKey_XXXX.p8 | pbcopy`, `gh secret set NAME --env release` as an alternative to the web UI, the tag push, and the `codesign -dv --verbose=4` / `spctl -a -vvv -t install` / `brew install lupguo/tap/landrop` checks.
  - Chapter 8 does an rc rehearsal (`v0.1.0-rc.1`) first, then `v0.1.0`. It includes a first-release checklist that runs install.ps1 on a real Windows machine and install.sh on a real Linux machine.
  - Chapter 9 covers deleting a bad tag (`git push --delete origin vX`, delete the GitHub Release) and reverting a tap commit.
  - Mark security warnings with a `> ⚠️ 安全提醒` callout at each step where a secret is created, stored or used. Chapter 10 collects all of them plus the revoke steps.
  - Chapter 0 includes `brew install gitleaks && gitleaks git -v .` and states that making the repo public cannot be undone in effect.
- [ ] **Step 2: Verify.** Every secret name in `.github/workflows/release.yml` and `.goreleaser.yaml` appears in the manual. Run `for s in $(grep -ohE '(MACOS|HOMEBREW)_[A-Z0-9_]+' .github/workflows/release.yml .goreleaser.yaml | sort -u); do grep -q "$s" docs/release-setup.md || echo MISSING $s; done` → no output.
- [ ] **Step 3:** Run `git add docs/release-setup.md && git commit -m "docs: step-by-step release setup manual"`.

### Task 7: README (both languages) with screenshots

**Files:**
- Modify: `README.md`
- Commit: `imgs/web.jpg`, `imgs/cmd.jpg` (already staged)

- [ ] **Step 1:** Apply spec §4.6:
  - Place the screenshots side by side under the title and at the top of 中文说明. Use `<p align="center"><img src="imgs/web.jpg" alt="Web UI" width="49%"> <img src="imgs/cmd.jpg" alt="Terminal" width="49%"></p>`, or similar.
  - Add a new Install / 安装 section before Quick start, in the order brew → install.sh → install.ps1 → manual download with `shasum -a 256 -c checksums.txt --ignore-missing`.
  - Document `LANDROP_VERSION` / `LANDROP_INSTALL_DIR`, and add a one-line safety note about inspecting piped scripts.
  - Quick start becomes `landrop` after install.
  - Move the `make all` content into Build from source and add `make release-snapshot`.
  - The unsigned-binary section keeps only Windows SmartScreen and states that macOS builds are signed and notarized (first run checks online with Apple).
  - Mention `docs/release-setup.md` for maintainers.
- [ ] **Step 2: Verify.**
  - `grep -c 9958 README.md` ≥ 3, and `grep -n xattr README.md` → no output.
  - Both image paths exist.
  - Every command in the Install section matches the Global Constraints URLs exactly.
- [ ] **Step 3:** Run `git add README.md imgs/web.jpg imgs/cmd.jpg && git commit -m "docs: README install guide and product screenshots"`.

### Task 8: Final check

- [ ] Run `go test ./... && (cd web && npm test) && goreleaser check && actionlint && make release-snapshot && scripts/test-install.sh` → all pass.
- [ ] Run `git log --oneline main..` → one commit per task.
- [ ] Do not push, tag, or change repo visibility; the user does those by following `docs/release-setup.md`.
