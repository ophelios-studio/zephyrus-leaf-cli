# Embedded resources

Everything here gets baked into the Go binary via `go:embed`.

## `phar/`

The bundled `zephyrus-leaf-core` runtime, built at release time from a pinned core tag:

```
scripts/build-phar.sh  # composer install against leaf-core, box to leaf.phar
```

Not committed — populated by CI before the Go build.

## `scaffolds/`

Starter templates that `leaf init` unpacks into the user's new project.

- `docs-only/`       — just Markdown docs, default theme
- `docs-landing/`    — docs plus a Latte landing page
- `bare/`            — empty shell, user brings their own templates

Each scaffold is a real project tree mirroring what the binary expects at runtime:
`content/`, `templates/` (optional), `public/`, `config.yml`.
