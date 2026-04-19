# Plan: zephyrus-leaf-cli implementation

**Date**: 2026-04-19
**Status**: DRAFT, awaiting approval

---

## Goal

Build a Go binary (`leaf`) that ships Zephyrus Leaf as a zero-dependency static site CLI. No PHP, no Composer, no Docker on the user's machine. One self-contained binary per platform (macOS arm64/amd64, Linux arm64/amd64, Windows amd64). Users author Markdown (+ optional Latte/PHP/HTML templates and CSS/assets) and run `leaf init`, `leaf dev`, `leaf build`. Internally the binary embeds FrankenPHP's PHP runtime + a phar of `zephyrus-leaf-core`, and overlays the user's project directory on top of bundled defaults.

This is the "binary tier" in a two-tier distribution. The other tier is the existing `composer create-project dadajuice/zephyrus-leaf` path. Both consume the same `zephyrus-leaf-core` library. The binary hides PHP; the Composer tier exposes it fully for users who want to extend controllers, add Composer packages, or bend the framework.

## Approach

**PHP runtime**: FrankenPHP as a Go library. We instantiate the PHP interpreter in-process, execute the embedded phar, capture stdout/stderr. No Caddy server used. If FrankenPHP's library API proves awkward for pure-CLI use, fall back to linking `libphp` from static-php-cli directly (same static binary outcome, different embedding API).

**Build artifact**: `go:embed` bundles three things into the binary: (1) `leaf.phar` (zephyrus-leaf-core + vendor, baked at release), (2) scaffolds sourced from the `zephyrus-leaf` template repo at a pinned tag, (3) default Latte/asset bundles that overlay resolution falls back to.

**Dev server**: rebuild-to-`dist/` + Go static + WebSocket for live reload. Single rendering codepath (dev = prod). File watcher reuses `Leaf\FileWatcher` semantics. No PHP-per-request surprises.

**Overlay resolution**: done Go-side with a temp-dir merge. At build start, the Go binary copies embedded defaults into `$TMP/project/`, then copies the user's `./templates/`, `./public/`, `./content/`, `./config.yml` over the top. Leaf-core is pointed at `$TMP/project/` as its ROOT_DIR and sees a normal project layout, unaware that half the files came from the binary. No upstream leaf-core or zephyrus2 changes required. On macOS we use APFS clonefile for near-zero copy cost; on Linux, reflink or hardlinks; on Windows, plain copy (acceptable given site sizes).

**Scaffolds**: sourced at release time from `ophelios-studio/zephyrus-leaf` at a pinned tag. CI script clones, strips (`composer.json`, `vendor/`, `app/Controllers/`, `app/Models/`, `bin/`, `Kernel.php`, `docker-compose.yml`), and embeds. Single source of truth.

**Eject**: a one-way migration that writes `composer.json`, `app/` tree, and `bin/` into the user's project, reconstructing a full Composer-tier project from the binary-tier state. Documented as "advanced" and irreversible (they can re-init if regretted).

**Alternatives considered and rejected**:
- *Docker wrapper*: "halfway to nowhere" per prior discussion. Requires Docker daemon, bad beginner UX, worse release story.
- *Port to Go*: throws away the PHP content pipeline. Retype did this and now has to maintain their own Markdown + template engine. Not worth it.
- *Phar-only (no binary)*: still requires PHP installed. Same user friction.
- *FrankenPHP server mode for dev*: two codepaths (serve-on-demand vs batch-build) means dev/prod drift. Rebuild-per-change is fast enough.

## Implementation milestones

Each milestone is independently mergeable. Tests ship **with** the milestone they belong to, not after.

### M0 — Scaffold (DONE, commit 030cd33)

Repo structure, README, LICENSE, .gitignore, placeholder main.go, placeholder scripts. Already live at https://github.com/ophelios-studio/zephyrus-leaf-cli (private).

### M1 — `leaf build` end-to-end ✅ (core path working; phar packaging deferred to M5)

**Deliverable**: `./leaf build` in a directory with `content/` and `config.yml` produces `dist/` HTML. No init, no dev, no release polish yet.

**Go work**:
1. `internal/runtime/php.go`: thin wrapper around FrankenPHP library. `Run(phar string, args []string, env map[string]string, cwd string) error`. Captures stdout/stderr.
2. `internal/phar/embed.go`: `go:embed` leaf.phar, expose as a `fs.FS` or temp-extract to a deterministic path.
3. `internal/overlay/resolver.go`: given user project root + embedded default root, produce the effective path list (user wins). Unit-tested against table of fixtures.
4. `internal/project/config.go`: read `./config.yml`, validate minimum fields, resolve relative paths.
5. `cmd/leaf/build.go`: wire it all up. Call `runtime.Run(embeddedPhar, ["build"], env, userCwd)`.

**Phar build** (M5 owns the final pipeline, but M1 needs one to develop against):
- `scripts/build-phar.sh`: clone leaf-core at pinned tag, `composer install --no-dev`, box to `internal/embed/phar/leaf.phar`. Run locally for now.

**Tests**:
- Unit (Go):
  - `overlay_test.go`: table-driven, 8-10 cases covering user-override, fallback, missing-both-error.
  - `config_test.go`: valid YAML, minimal YAML, broken YAML, missing required fields.
  - `runtime_test.go`: mocked PHP call, arg/env/cwd propagation.
- Integration (Go):
  - `testdata/minimal-site/` fixture with `content/getting-started/intro.md` + `config.yml`.
  - Test runs the real binary against the fixture, asserts `dist/getting-started/intro/index.html` exists and contains the rendered title.
  - Same fixture with a user `templates/layouts/docs.latte` override, asserts that template's marker string appears in output.
- Contract:
  - `phar_contract_test.go`: assert the embedded phar exposes `Leaf\BuildCommand`. If leaf-core renames it, this test fails loudly.

**Files affected**:
| File | Change |
|---|---|
| `cmd/leaf/main.go` | Dispatch `build` subcommand |
| `cmd/leaf/build.go` | New |
| `internal/runtime/php.go` | New |
| `internal/runtime/php_test.go` | New |
| `internal/phar/embed.go` | New |
| `internal/overlay/resolver.go` | New |
| `internal/overlay/resolver_test.go` | New |
| `internal/project/config.go` | New |
| `internal/project/config_test.go` | New |
| `internal/embed/phar/leaf.phar` | Built, gitignored |
| `scripts/build-phar.sh` | Real implementation |
| `test/integration/build_test.go` | New |
| `test/integration/testdata/minimal-site/` | New fixture |
| `test/integration/testdata/override-site/` | New fixture |

### M2 — `leaf init` with embedded scaffolds

**Deliverable**: `leaf init my-site --template=docs-only|docs-landing|bare` creates a working project.

**Go work**:
1. `internal/scaffolds/embed.go`: `go:embed` three scaffold trees from `internal/embed/scaffolds/{docs-only,docs-landing,bare}/`.
2. `internal/scaffolds/extract.go`: walk embedded fs, write files into target dir. Refuse to overwrite unless `--force`. Empty-dir check.
3. `cmd/leaf/init.go`: parse `<name>` positional + `--template=` flag, default to `docs-only`. Print next-steps (cd, leaf dev).

**Scaffold sourcing pipeline** (CI-only, owned by M5 but prototype here):
- `scripts/build-scaffolds.sh`: clone zephyrus-leaf at pinned tag, for each template variant strip appropriate files, copy result into `internal/embed/scaffolds/<variant>/`.
- For M2, run it once locally and commit the output so we can develop against it. M5 automates this in CI per release.

**Tests**:
- Unit:
  - `extract_test.go`: write to tmpdir, assert file tree matches expected.
  - Refuse-overwrite behavior: create target, pre-populate with file, assert error without `--force`, assert success with `--force`.
- Integration:
  - `init_test.go`: run `leaf init tmpdir --template=docs-only`, then `leaf build` against the result. End-to-end: init → build.
  - Same for `docs-landing` — assert landing.latte present and rendered.
  - Same for `bare` — assert minimal config + empty content/, build succeeds (even with no pages).

**Files affected**:
| File | Change |
|---|---|
| `cmd/leaf/init.go` | New |
| `internal/scaffolds/embed.go` | New |
| `internal/scaffolds/extract.go` | New |
| `internal/scaffolds/extract_test.go` | New |
| `internal/embed/scaffolds/docs-only/` | New (generated) |
| `internal/embed/scaffolds/docs-landing/` | New (generated) |
| `internal/embed/scaffolds/bare/` | New (generated) |
| `scripts/build-scaffolds.sh` | Real implementation |
| `test/integration/init_test.go` | New |

### M3 — `leaf dev` with live reload

**Deliverable**: `leaf dev` in a project dir starts a server on `:8080`, rebuilds on file change, browser auto-reloads.

**Go work**:
1. `internal/devserver/server.go`: Go `net/http` serving from the user's `dist/` dir. Injects a WebSocket reload snippet into every `*.html` response.
2. `internal/devserver/watcher.go`: fsnotify-based watcher on `content/`, `templates/`, `public/`, `config.yml`. Debounced 200ms.
3. `internal/devserver/reload.go`: WebSocket broadcaster. On file change: trigger rebuild, on rebuild success: send "reload" to all clients.
4. `cmd/leaf/dev.go`: wire together. On start: run build once, start server, start watcher.

**Tests**:
- Unit:
  - `watcher_test.go`: create tmpdir with files, start watcher, touch files, assert events delivered (with debounce).
  - `reload_test.go`: two mock WS clients, broadcast message, both receive it.
  - `inject_test.go`: given HTML string, injection places the reload script before `</body>`.
- Integration:
  - `dev_test.go`: start dev server in a goroutine against a fixture, `http.Get("/")` returns 200 + expected HTML. Modify a content file, assert rebuild happens within 1s, next GET shows updated content.
  - WebSocket subscribe via a test client, modify file, assert reload event received.

**Files affected**:
| File | Change |
|---|---|
| `cmd/leaf/dev.go` | New |
| `internal/devserver/server.go` | New |
| `internal/devserver/server_test.go` | New |
| `internal/devserver/watcher.go` | New |
| `internal/devserver/watcher_test.go` | New |
| `internal/devserver/reload.go` | New |
| `internal/devserver/reload_test.go` | New |
| `internal/devserver/inject.go` | New |
| `internal/devserver/inject_test.go` | New |
| `test/integration/dev_test.go` | New |

### M4 — `leaf eject`

**Deliverable**: `leaf eject` converts the user's binary-tier project into a Composer-tier project.

**Go work**:
1. `internal/eject/embed.go`: `go:embed` the "Composer addendum" tree (composer.json, app/Controllers/, app/Kernel.php, bin/, public/index.php, public/router.php, the bits the binary was hiding).
2. `cmd/leaf/eject.go`: confirm-prompt, write addendum into cwd, print next steps (`composer install`, `composer dev`).

**Tests**:
- Integration:
  - `eject_test.go`: init a docs-landing project, eject, assert composer.json present, assert app/Kernel.php present. If Composer is available in CI, run `composer install` and then `bin/build.php`, assert `dist/` populated.
  - Without composer in env, assert the printed next-steps mention it.

**Files affected**:
| File | Change |
|---|---|
| `cmd/leaf/eject.go` | New |
| `internal/eject/embed.go` | New |
| `internal/embed/addendum/` | New (generated from zephyrus-leaf at the same pinned tag) |
| `test/integration/eject_test.go` | New |

### M5 — Release pipeline + install script

**Deliverable**: tagging `v1.0.0` produces signed per-platform binaries + a GitHub Release. Install script live at `leaf.ophelios.com/install.sh`.

**Work**:
1. `scripts/build-phar.sh`: real version. Clones leaf-core at the tag pinned in `LEAF_CORE_VERSION`, builds phar, places in `internal/embed/phar/`.
2. `scripts/build-scaffolds.sh`: real version. Clones zephyrus-leaf at pinned tag, produces scaffold variants, places in `internal/embed/scaffolds/`.
3. `scripts/build-matrix.sh`: build `leaf-<os>-<arch>[.exe]` for all 5 targets via FrankenPHP static build. Each target needs its own build env; for macOS + Linux we can cross-compile from a Linux host with target toolchains; Windows needs a Windows runner.
4. `.github/workflows/release.yml`: matrix job, triggered by `v*` tags. For each target:
   - Check out repo at tag
   - Build phar + scaffolds
   - Build static binary
   - Upload to release
   - Publish checksums

   No code signing in v1.0. Rationale: the primary install path is `curl ... | sh`, and `curl` does not set `com.apple.quarantine` xattr, so Gatekeeper never checks. Same for Windows SmartScreen via PowerShell install. Users who download from the Releases page via browser get a documented `xattr -d com.apple.quarantine $(which leaf)` one-liner in the README. Revisit signing if real user complaints surface.
5. `scripts/install.sh`: bash script that detects OS/arch, downloads the right binary from releases, verifies checksum, installs to `/usr/local/bin/leaf` (or user-provided prefix). Hosted at `leaf.ophelios.com/install.sh` (served from `zephyrus-leaf-site` `public/` as a static file).

**Tests**:
- CI runs the full integration + e2e suite on **every platform in the matrix**:
  - macos-14 (arm64), macos-13 (amd64), ubuntu-latest (amd64), ubuntu-22.04-arm (arm64), windows-latest (amd64).
  - Each runner: build, run `leaf init && leaf build && grep expected_string dist/*/index.html`.
- Install script smoke test: after each release, a separate workflow curls the install script on fresh containers for each OS, runs `leaf version`, asserts the version matches the tag.
- Size budget check: fail if any binary exceeds 120MB (allows headroom above FrankenPHP's ~40MB base).

**Files affected**:
| File | Change |
|---|---|
| `scripts/build-phar.sh` | Real impl |
| `scripts/build-scaffolds.sh` | New |
| `scripts/build-matrix.sh` | Real impl |
| `scripts/install.sh` | New |
| `.github/workflows/release.yml` | Real impl |
| `.github/workflows/ci.yml` | New (unit + integration on push) |
| `.github/workflows/install-smoke.yml` | New (post-release) |
| `LEAF_CORE_VERSION` | New, plain-text file holding the pinned core tag |

### M6 — Docs + polish

**Deliverable**: a user can land on the docs site and succeed.

**Work**:
1. Add a "Binary" section to `zephyrus-leaf-site` (the docs site this conversation is about): install script, quickstart, `leaf` command reference, eject path, FAQ.
2. Landing page CTA adjustment: `curl ... | sh` alongside the existing `composer create-project` pill.
3. Changelog convention (Keep-a-Changelog format) in the CLI repo.

**Tests**:
- No new test code, but validate: docs examples are copy-paste runnable on each platform.

---

## Test strategy summary

| Layer | Tool | Where | Runs when |
|---|---|---|---|
| PHP unit | PHPUnit | `zephyrus-leaf-core/tests/Unit/` | on push to core |
| Go unit | stdlib `testing` | `internal/*/_test.go` | on push to cli |
| Go integration | stdlib `testing` + `os/exec` | `test/integration/` | on push to cli, all platforms |
| Contract | Go test asserting phar exposes known entry points | `internal/phar/contract_test.go` | on push; fails if core drifts |
| Install smoke | Shell + GitHub Actions containers | separate workflow | after each release tag |
| Size budget | Shell check in release workflow | `.github/workflows/release.yml` | in release pipeline |

**Platform matrix (mandatory for green release)**: macos-14, macos-13, ubuntu-latest, ubuntu-22.04-arm, windows-latest. A release cannot publish if any platform's integration suite fails.

**No mocking of PHP**. The integration tests run the real embedded runtime against real fixtures. Mocking would let dev/prod drift happen — exactly the risk the CLI exists to eliminate.

**Fixtures**: all under `test/integration/testdata/`. Three fixtures minimum: `minimal-site` (one doc page, no templates), `override-site` (user overrides one layout template), `landing-site` (custom landing.latte at root). Add more as bugs are reported.

---

## Files to change

### New files (in `zephyrus-leaf-cli`)
Full scaffold in the tree above. Most work lands in `cmd/leaf/`, `internal/`, `scripts/`, `test/integration/`, `.github/workflows/`.

### New files (in `zephyrus-leaf-site`)
| File | Reason |
|---|---|
| `public/install.sh` | The install script served at leaf.ophelios.com/install.sh |
| `content/<section>/binary.md` | Docs for the binary tier |

---

## Files NOT to change (out of scope)

- The existing `composer create-project` tier UX in `zephyrus-leaf`. It continues to work exactly as today.
- The PHP runtime path of rendering. We invoke the same `BuildCommand::run` the Composer tier uses.
- The Markdown, Latte, or content pipeline. The binary changes *how PHP is distributed*, not *what PHP does*.
- Tests in `zephyrus-leaf` (the composer template). That repo has none currently and the CLI doesn't require it to add any; its smoke testing lives as the "landing-site" CI fixture in CLI.

---

## Risks & edge cases

**FrankenPHP library API ergonomics (medium, likely).** FrankenPHP is primarily marketed as a web server. Its Go library API is stable but not designed for CLI-dispatch usage. Possible friction: stdin/stdout routing, signal handling (Ctrl-C in `leaf dev`), PHP_INI overrides. *Mitigation*: Spend the first 2 days of M2 building a "hello world" minimal prototype that invokes `phpinfo()` via the embed API before committing to the whole plan. If blocked, swap to static-php-cli's `libphp` + `cgo` — same static binary outcome, different embedding code.

**Cross-compile complexity for Windows (medium, likely).** FrankenPHP on Windows is less battle-tested. Requires a Windows build runner, MSVC toolchain, and authenticode signing cert. *Mitigation*: plan for a real Windows CI runner in M6 from day one. Don't attempt to cross-compile Windows from Linux.

**Binary size (low, unlikely).** FrankenPHP ~40MB, our phar + scaffolds add ~5-10MB, vendor libs ~5MB. Realistic target: 50-70MB per binary. Size budget gate at 120MB catches runaway growth.

**macOS Gatekeeper for browser-downloaded binaries (low).** The primary install path is `curl ... | sh`, which does not set `com.apple.quarantine` and therefore bypasses Gatekeeper. Only users who click the asset on the GitHub Releases page in a browser hit the "cannot verify developer" warning. *Mitigation*: README one-liner (`xattr -d com.apple.quarantine $(which leaf)`). Revisit full notarization (~$99/yr Apple Developer account) only if complaints pile up post-launch. Same reasoning deferred Windows authenticode signing.

**Scaffold drift (low).** If the zephyrus-leaf template adds a feature that requires a new file in `app/`, the binary's scaffolds need updating. *Mitigation*: the release pipeline rebuilds scaffolds from the pinned tag every release; drift is detectable. Add a CI check that diffs the embedded scaffolds against a freshly-sourced copy — fail if different without an explicit update of `LEAF_CORE_VERSION`.

**User confusion between tiers (ongoing, medium).** Two ways to install, different capabilities. *Mitigation*: docs lead with "most users want the binary; pick Composer if you need X, Y, Z." The eject command makes the upgrade path obvious.

**Leaf-core breaking changes post-pin (medium).** If core v1.3 ships but the pinned CLI is on core v1.2, users filing bugs may hit fixed-upstream issues. *Mitigation*: Contract test (mentioned in M2) catches API drift. `LEAF_CORE_VERSION` is the single source of truth; bumping it is always a deliberate commit.

**Windows line endings in scaffolds.** If scaffolds are generated on Linux CI and unpacked on Windows, CRLF vs LF matters for text files. *Mitigation*: embed with explicit byte preservation; do not rewrite at extract time.

**Live-reload WebSocket on Windows.** fsnotify on Windows has known event coalescing quirks. *Mitigation*: standard 200ms debounce + content-hash equality check (already the pattern in leaf-core's FileWatcher).

---

## Open questions

1. ~~**Leaf-core overlay PR**~~ **RESOLVED**: no upstream change needed. Go-side temp-dir merge handles overlay without touching leaf-core or zephyrus2.
2. ~~**macOS Apple Developer account**~~ **RESOLVED**: v1.0 ships unsigned. `curl | sh` bypasses Gatekeeper.
3. **Install script host**: `leaf.ophelios.com/install.sh` (served from zephyrus-leaf-site's `public/`)? Default yes unless there's a reason to separate.
4. **Go module path**: `github.com/ophelios-studio/zephyrus-leaf-cli` is locked in the scaffold. Happy to keep.
5. **v1.0 criteria**: all 5 platforms green in CI, install script stable, docs section complete, 3 dogfooded projects.

---

## Definition of Done (for the whole plan, not per milestone)

- [ ] `curl -fsSL https://leaf.ophelios.com/install.sh | sh` installs a working `leaf` binary on macOS, Linux, Windows
- [ ] `leaf init my-docs && cd my-docs && leaf build` produces `dist/` with expected HTML on all 5 platforms
- [ ] `leaf dev` serves at `:8080`, reloads on content/template/asset change, on all 5 platforms
- [ ] `leaf eject` produces a functional Composer project that `composer install && composer build` succeeds
- [ ] Integration + unit test suite passes on all 5 platforms in CI
- [ ] Binary size under 120MB per platform
- [ ] Docs section in zephyrus-leaf-site live and accurate
- [ ] `LEAF_CORE_VERSION` contract test passes against the embedded phar
- [ ] Release artifact checksums published with every release
