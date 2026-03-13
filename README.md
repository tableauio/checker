# checker
The configuration checker for Tableau.

## Prerequisites

- Init project: `bash init.sh`

## Run

1. Generate `*.pb.go` and `*.check.go`: `bash test/gen.sh`
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

## Third Party

Submodules dependency:
- **loader**: `loader/cmd/protoc-gen-go-tableau-loader`
- **tableau**: `tableau/proto/tableau.proto`
- **protobuf**: `protobuf/src/*.proto` and `protoc`
