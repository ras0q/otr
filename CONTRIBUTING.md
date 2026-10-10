# Contributing

## Layout

- **`app/`** — domain and use cases (protocol-agnostic).
- **`inbound/<adapter>/<implement>/`** — incoming adapters (e.g. npm registry).
- **`outbound/<adapter>/<implement>/`** — outgoing ports (e.g. storage, source).
- **`main.go`** — wiring only.

**Adapter** = capability name (`storage`, `source`, `registry`).
**Implement** = concrete backend or protocol (`local`, `s3`, `github`, `npm`).

Put shared interfaces and types in `<adapter>/` (sibling of `<implement>/`), e.g. `outbound/storage/storage.go`.

## Conventions

| Item | Rule | Example |
|------|------|---------|
| Path | `{inbound\|outbound}/<adapter>/<implement>/` | `outbound/storage/local/` |
| Entry file | `<implement>_<adapter>.go` | `local_storage.go` |
| Package | `<implement>` | `package local` |
| Constructor | `New<Adapter>` | `local.NewStorage` |

```go
// outbound/storage/local/local_storage.go
package local

func NewStorage(...) (*Store, error) { ... }
```

Implementations should satisfy the parent interface (e.g. `var _ storage.Storage = (*Store)(nil)`).

## Dependencies

- `inbound/*` → `app/*`, outbound **interfaces** only (concrete types from `main`).
- `outbound/*` → `app/*` when needed.
- `app/*` must not import concrete `inbound/` or `outbound/<implement>/` packages.

## Tests

Keep `*_test.go` next to the code under test. Protocol e2e fixtures may live under the implement tree (e.g. `inbound/registry/npm/e2e/`).
