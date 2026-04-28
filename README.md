# checker
The configuration checker for Tableau.

## Prerequisites

- [Go](https://go.dev/doc/install) (>= 1.24)
- [buf](https://buf.build/docs/installation) (>= v1.40)

## Run

1. Generate `*.pb.go` and `*.check.go`: `cd test && buf generate`
2. Test: `cd test && go test ./...`
3. Run directly: `cd test && go run .`

## Lint
We suggest to use `strict` mode for detecting and excluding auto-generate files in checker project.
An example `.golangci.yaml` is as follows:
```yaml
version: "2"
linters:
  exclusions:
    generated: strict
```

## Code Generation

Code generation is driven by [buf](https://buf.build/) via the
configuration under `test/`:

- `test/buf.yaml`: module and remote dependencies (e.g. `buf.build/tableauio/tableau`).
- `test/buf.gen.yaml`: plugins used to generate code, including:
  - `buf.build/protocolbuffers/go` — generates `*.pb.go`.
  - `github.com/tableauio/loader/cmd/protoc-gen-go-tableau-loader` — generates loader code.
  - `protoc-gen-go-tableau-checker` (this repo) — generates `*.check.go`.
