# Leaf

A zero-dependency static site CLI.

Write Markdown. Optionally a landing in Latte, PHP, or HTML. Run `leaf build`. Deploy anywhere static.

The shipping goal: no PHP, no Composer, no Docker on the user's machine. One self-contained binary per platform.

## Status

All four subcommands working, two build modes, release pipeline in place:

- `leaf init <name>` — scaffold a new site
- `leaf build` — render Markdown + Latte to static HTML
- `leaf dev` — live-reload server with file watcher and SSE-based reload
- `leaf eject` — convert binary-tier project into the Composer-tier

Build modes:

- **Standalone** (`-tags embed_defaults`): framework baked into the binary. Single ~13 MB executable per platform. Still needs system PHP at runtime (FrankenPHP static link is the final step).
- **Dev** (default): framework resolved from `LEAF_DEFAULTS_DIR`. For hacking on the CLI itself.

Releases:

- CI: Unit + embed build + integration on Ubuntu, macOS, Windows on every push.
- Release workflow: tag `v*` → cross-compiles 5 targets → publishes GitHub Release with checksums.
- Install script: [`scripts/install.sh`](scripts/install.sh), hosted at `leaf.ophelios.com/install.sh` (pending docs site deploy).

Not yet shipped:

- FrankenPHP static link (M5 final step), so users don't need system PHP.

## Install (planned)

```sh
curl -fsSL https://leaf.ophelios.com/install.sh | sh
```

## Quickstart (dev mode, today)

```sh
git clone https://github.com/ophelios-studio/zephyrus-leaf-cli
cd zephyrus-leaf-cli
go build -o bin/leaf ./cmd/leaf

# Point the binary at a local zephyrus-leaf checkout (with vendor installed).
export LEAF_DEFAULTS_DIR=/path/to/zephyrus-leaf

./bin/leaf init my-docs
cd my-docs
../bin/leaf dev            # serves at http://localhost:8080 with live reload
../bin/leaf build          # writes dist/
```

## Project structure (what users see)

```
my-site/
├── content/           # markdown docs, sections, pages
├── templates/         # optional: Latte/PHP/HTML overrides
├── public/            # static assets copied verbatim
├── config.yml         # site config
└── dist/              # build output (gitignored)
```

No `app/`, no `vendor/`, no `Kernel.php`. The framework lives inside the binary.

## Templating

- `templates/layouts/docs.latte` overrides the bundled docs layout.
- `templates/partials/nav.latte` overrides the bundled nav. Any path under `templates/` maps to `app/Views/<same path>` inside the build.
- Landing page: drop a `templates/landing.latte` (or `.php`, or `.html`). If present, it renders at `/`. If absent, `/` redirects to the first doc page.

Pick what fits. Docs theme comes from bundled defaults; override any template by placing a file at the same path.

## Architecture

Every `leaf build` follows the same pipeline:

1. Load `config.yml` from the project root.
2. Create a tempdir, overlay-merge the embedded framework defaults + the user's files into it.
3. Invoke PHP (system `php` in dev mode, statically-linked FrankenPHP in release) against `bin/build.php` inside that tempdir.
4. Copy the rendered `dist/` back into the project root.
5. Clean up the tempdir.

`leaf dev` does step 1-4 on every file change and broadcasts an SSE "reload" message to every connected browser.

Overlay precedence: user's files win over bundled defaults. `content/` and `config.yml` are user-exclusive (defaults don't leak through). `templates/` and `public/` are file-level overrides (defaults survive where the user didn't replace).

## Want more?

For advanced use (custom controllers, Composer packages, framework extension), use the [Composer template](https://github.com/ophelios-studio/zephyrus-leaf) instead:

```sh
composer create-project dadajuice/zephyrus-leaf my-site
```

Same core (`zephyrus-leaf-core`), richer escape hatches.

Or start with the binary and run `leaf eject` when you outgrow it. One-way, documented.

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md).

## License

MIT.
