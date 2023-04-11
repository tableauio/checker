# checker
The configuration checker for Tableau.

## Prerequisites

- Init project: `bash init.sh`

## Run

- Generate `*.pb.go` and `*.pc.go`: `bash scripts/gen.sh`
- Build: `go build`
- Run: `./checker`

## Third Party

Submodules dependency:
- **loader**: `loader/cmd/protoc-gen-go-tableau-loader`
- **tableau**: `tableau/proto/tableau.proto`
- **protobuf**: `protobuf/src/*.proto` and `protoc`
