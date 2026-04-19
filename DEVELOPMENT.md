# Development

## Prerequisites

- Go 1.22+
- PHP 8.4+ with `ext-intl`, `ext-mbstring` (only for integration tests and running `leaf build` in dev mode)
- A local checkout of [`zephyrus-leaf`](https://github.com/ophelios-studio/zephyrus-leaf) with `composer install` completed (only for integration tests)

## Build

Two build modes, selected via tag:

```sh
# Dev mode: LEAF_DEFAULTS_DIR must point at a local zephyrus-leaf checkout.
# Fast Go rebuilds, no framework baked into the binary.
go build -o bin/leaf ./cmd/leaf

# Embedded mode: framework compiled into the binary. Standalone after build
# (still shells out to system `php` until M5 wires FrankenPHP).
go run ./scripts/stage -src /path/to/zephyrus-leaf
go build -tags embed_defaults -o bin/leaf ./cmd/leaf
```

In dev mode the binary shells out to the system `php` and uses the framework scaffold from `LEAF_DEFAULTS_DIR`. Embed mode bakes the framework via `go:embed`. Release mode (M5) will additionally embed FrankenPHP so no system PHP is required.

## Running the CLI locally

```sh
export LEAF_DEFAULTS_DIR=/path/to/zephyrus-leaf   # local clone with vendor installed
./bin/leaf build --dir /path/to/your/site
```

Your site needs at minimum a `config.yml` and a `content/` directory.

## Testing

```sh
# Unit tests, fast, no PHP needed.
go test ./internal/... ./cmd/...

# Integration tests, require PHP + LEAF_DEFAULTS_DIR.
LEAF_DEFAULTS_DIR=/path/to/zephyrus-leaf go test ./test/integration/
```

Integration tests skip gracefully when PHP or the defaults dir isn't available.

## Project layout

```
cmd/leaf/              CLI entry point; thin dispatch to internal packages
internal/
  overlay/             Tempdir merge of embedded defaults + user overrides
  project/             Config loader, defaults-source resolver
  runtime/             PHPRuntime interface with exec + frankenphp + mock impls
test/integration/      End-to-end tests running the real binary against fixtures
scripts/               Release-time build scripts (phar, scaffolds, matrix)
.github/workflows/     CI and release pipelines
.claude/plans/         Plan files driving implementation (commit with feature)
```

## Build tags

- default (no tags): uses `internal/runtime/exec.go`, shells out to system `php`.
- `-tags embed_php`: uses `internal/runtime/frankenphp.go`. Requires a statically-linked libphp and the FrankenPHP build environment. Production only.

## Contributing

Commits follow the project convention: semantic prefix (`feat:`, `fix:`, `docs:`, `chore:`, `refactor:`, `test:`), single-line message, no co-authors. Tests ship with the feature they cover.
