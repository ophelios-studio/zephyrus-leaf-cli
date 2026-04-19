# Leaf

A zero-dependency static site CLI.

Write Markdown. Optionally a landing in Latte, PHP, or HTML. Run `leaf build`. Deploy anywhere static.

No PHP. No Composer. No Docker. One binary per platform, self-contained.

## Status

Early. Not yet released.

## Install (planned)

```sh
curl -fsSL https://leaf.ophelios.com/install.sh | sh
```

Or download a binary from [Releases](https://github.com/ophelios-studio/zephyrus-leaf-cli/releases).

## Quickstart (planned)

```sh
leaf init my-docs           # scaffold a new site
cd my-docs
leaf dev                    # serve at localhost:8080 with live reload
leaf build                  # generate static HTML into dist/
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

`templates/landing.latte` → rendered at `/` with full Latte power (layouts, blocks, partials).
`templates/landing.php` → plain PHP view, same variables injected.
`templates/landing.html` → copied through. No interpolation. Bring your own CSS.

Pick what fits. Docs theme comes from bundled defaults; override any template by placing a file at the same path.

## Want more?

For advanced use (custom controllers, Composer packages, framework extension), use the [Composer template](https://github.com/ophelios-studio/zephyrus-leaf) instead:

```sh
composer create-project dadajuice/zephyrus-leaf my-site
```

Same core (`zephyrus-leaf-core`), richer escape hatches.

## Eject

Outgrew the binary? `leaf eject` writes a full `composer.json` and `app/` tree into your project, converting it to the full Composer path. One-way migration, documented.

## Architecture

- Go wrapper embedding [FrankenPHP](https://frankenphp.dev) (PHP as a statically-linked library).
- PHP app bundled as a phar at release time from [`zephyrus-leaf-core`](https://github.com/ophelios-studio/zephyrus-leaf-core).
- Scaffolds embedded via `go:embed`.
- Cross-compiled per platform in CI.

## Versioning

`leaf-cli v1.2.0` pins `zephyrus-leaf-core ^1.2`. Tag core → tag CLI → release pipeline bakes the phar from that core tag and publishes per-platform binaries.

## License

MIT.
