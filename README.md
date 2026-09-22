# checker

Configuration checker for [Tableau](https://github.com/tableauio/tableau).

This repository provides **`protoc-gen-go-tableau-checker`**, a `protoc` /
[buf](https://buf.build/) plugin that generates `*.check.go` stubs. You implement
`Check` / `CheckCompatibility` on those stubs; the generated hub loads configs
and runs the checks (including `ProcessAfterLoadAll` for derived / custom
messagers).

## Prerequisites

- [Go](https://go.dev/doc/install) (>= 1.24, see `go.mod`)
- [buf](https://buf.build/docs/installation) (>= v1.40; CI uses 1.67.0)

## Develop / test

The `test/` directory is an **integration suite** (proto → generate → `go test`).
It has no `main` package, so `cd test && go run .` does **not** work.

From the repo root:

```bash
# 1) Regenerate *.pb.go, loader *.pc.go, and *.check.go under test/
cd test && buf generate && cd ..

# 2) Run all tests (plugin unit tests + test/ examples & checks)
go test ./...

# 3) Run documented examples under test/ (stdout is matched to // Output:)
go test ./test/ -run 'Example_' -count=1
```

Useful narrower commands:

```bash
go test ./cmd/protoc-gen-go-tableau-checker/   # plugin AST / generator tests
go test ./test/check/                         # checker hub + customconf tests
go test ./test/ -run Example_customConf -v    # single example (PASS still hides Output)
```

## Use the plugin

Wire it into `buf.gen.yaml` (see `test/buf.gen.yaml` for a working local setup):

```yaml
version: v2
plugins:
  - local: ["go", "run", "github.com/tableauio/checker/cmd/protoc-gen-go-tableau-checker@v0.7.1"]
    out: check
    opt:
      - paths=source_relative
      - out=check   # directory of existing *.check.go for incremental merge
    strategy: all
```

During local development against this repo (or before a release tag exists), point `local` at the tree instead:

```yaml
- local: ["go", "run", "../cmd/protoc-gen-go-tableau-checker"]
```

Common options:

| Option | Default | Meaning |
|--------|---------|---------|
| `pkg` | `check` | Go package name of generated checker files |
| `loader-pkg` | `tableau` | Loader package name under each file’s Go import path |
| `out` | _(empty)_ | Existing checker output dir used for incremental updates |

## Layout

| Path | Role |
|------|------|
| `cmd/protoc-gen-go-tableau-checker/` | Plugin source + embedded `hub` / `error` templates |
| `test/proto/` | Sample Tableau workbooks (`.proto`) |
| `test/protoconf/` | Generated `*.pb.go` + loader `*.pc.go` |
| `test/check/` | Generated / hand-edited `*.check.go` hub and checkers |
| `test/customconf/` | Example derived messager (`CustomItemConf`) |
| `test/testdata*/` | JSON / CSV fixtures for load & compatibility examples |
| `test/example_test.go` | Runnable `Example_*` demos |

## Lint

Prefer golangci-lint **strict** generated-file detection so `*.check.go` /
`*.pc.go` are excluded. Example `.golangci.yaml`:

```yaml
version: "2"
linters:
  exclusions:
    generated: strict
```

## Code generation details

`test/buf generate` runs three plugins (see `test/buf.gen.yaml`):

1. `buf.build/protocolbuffers/go` → `test/protoconf/*.pb.go`
2. `protoc-gen-go-tableau-loader` → `test/protoconf/tableau/*.pc.go`
3. `protoc-gen-go-tableau-checker` (this repo) → `test/check/*.check.go`

Hand-written logic in `*.check.go` (for example `ActivityConf.Check`) is kept
across regenerations via incremental AST merge when `out=check` points at the
existing checker directory.
