# checker
The configuration checker for Tableau.

## Prerequisites

- Init project: `bash init.sh`

## Run

- Generate `*.pb.go` and `*.pc.go`: `bash scripts/gen.sh`
- Build: `go build`
- Run: `./checker`

## Lint
We suggest to use `strict` mode for detecting and excluding auto-generate files in checker project. Manually disabling `gochecknoinits` is also required.
An example `.golangci.yaml` is as follows:
```yaml
version: "2"
linters:
  exclusions:
    generated: strict
    rules:
    - path: \.check\.go$
      linters:
        - gochecknoinits
```

## Third Party

Submodules dependency:
- **loader**: `loader/cmd/protoc-gen-go-tableau-loader`
- **tableau**: `tableau/proto/tableau.proto`
- **protobuf**: `protobuf/src/*.proto` and `protoc`
