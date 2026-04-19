# Leaf CLI

A zero-dependency static site CLI. Write Markdown. Optionally templates in Latte, PHP, or plain HTML. Run `leaf build`. Deploy anywhere static.

One binary per platform. No PHP or Composer to install on your machine at install time. (The binary currently shells out to system `php` at build time; FrankenPHP static link to remove that is on the roadmap.)

## Install

```sh
curl -fsSL https://leaf.ophelios.com/install.sh | sh
```

Detects your OS/arch, downloads the matching binary from [GitHub Releases](https://github.com/ophelios-studio/zephyrus-leaf-cli/releases/latest), verifies the checksum, installs to `/usr/local/bin/leaf`. Override with `LEAF_PREFIX` or `LEAF_VERSION`.

Supported platforms: macOS (arm64, amd64), Linux (arm64, amd64), Windows (amd64).

## Quickstart

```sh
leaf init my-docs
cd my-docs
leaf dev       # live preview on http://localhost:8080
leaf build     # writes dist/, ready to deploy
```

## Commands

```
leaf init <name>  Scaffold a new site
leaf dev          Serve with live reload
leaf build        Generate static HTML into dist/
leaf eject        Convert to the Composer project path (one-way)
leaf version
leaf help
```

Full reference: [leaf.ophelios.com/getting-started/cli-reference](https://leaf.ophelios.com/getting-started/cli-reference).

## Project structure

After `leaf init`:

```
my-site/
├── content/     # markdown pages, organized by section
├── templates/   # optional Latte/PHP/HTML overrides (add as needed)
├── public/      # static assets copied verbatim
├── locale/      # optional translation JSON files
├── config.yml   # site configuration
└── dist/        # build output (gitignore this)
```

No `app/`, `bin/`, `vendor/`, or `composer.json`. The framework lives inside the binary.

## Templating

Drop files under `templates/` to override the bundled theme. Paths map to `app/Views/<same path>` internally:

- `templates/layouts/docs.latte` — override the docs layout
- `templates/partials/nav.latte` — override the site nav
- `templates/landing.latte` — if present, renders at `/` instead of the default redirect
- Same patterns work with `.php` and `.html` (HTML is copied through without variable interpolation)

## Runtime requirement

At build time `leaf` needs system PHP 8.4+ with `intl`, `mbstring`, `sodium`, and `pdo` extensions. Removing this via FrankenPHP static link is the remaining milestone before true zero-dependency.

## Prefer PHP?

For teams that want custom controllers, Composer packages, or full ownership of `bin/build.php`, use the Composer template instead:

```sh
composer create-project zephyrus-framework/leaf my-docs
```

Same [`zephyrus-leaf-core`](https://github.com/ophelios-studio/zephyrus-leaf-core) under the hood.

Already started on the binary and want to switch? `leaf eject` converts the project (one-way).

## Architecture

Each `leaf build` runs:

1. Load `config.yml` from the project root.
2. Create a tempdir, overlay-merge the embedded framework + your `content/`, `templates/`, `public/`, `config.yml` into it.
3. Invoke PHP against `bin/build.php` inside the tempdir.
4. Copy the rendered `dist/` back into the project root.
5. Clean up.

`leaf dev` runs the same pipeline on every file change (250 ms debounced) and pushes Server-Sent Events to open browser tabs to auto-reload.

Overlay precedence: user files win over bundled defaults. `content/` and `config.yml` are user-exclusive (defaults don't leak through). `templates/`, `public/`, `locale/` are file-level overrides.

## Status

- `leaf init|dev|build|eject` ship and are tested on Linux, macOS, Windows on every push
- Pure-Go cross-compile (no CGO) produces ~13 MB binaries per platform
- Release pipeline: tag `v*` → cross-compile all 5 targets → publish GitHub Release with `checksums.txt`
- FrankenPHP static link (so users don't need system PHP): roadmap

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for building from source, running tests, and the release pipeline.

## License

MIT.
