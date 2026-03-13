#!/bin/bash

# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"

# Allow overriding protoc via environment variable.
# Default to locally compiled protoc for local development; fallback to system protoc.
LOCAL_PROTOC="./third_party/_submodules/protobuf/src/protoc"
if [ -z "$PROTOC" ]; then
    if [ -x "$LOCAL_PROTOC" ]; then
        PROTOC="$LOCAL_PROTOC"
    else
        PROTOC="$(which protoc 2>/dev/null || true)"
    fi
fi
if [ -z "$PROTOC" ]; then
    echo "Error: protoc not found. Please build protobuf submodule or install protoc." >&2
    exit 1
fi

# Allow overriding protobuf include path via environment variable.
# Default to local submodule source; fallback to system include path.
LOCAL_PROTOBUF_PROTO="./third_party/_submodules/protobuf/src"
if [ -z "$PROTOBUF_PROTO" ]; then
    if [ -d "$LOCAL_PROTOBUF_PROTO/google/protobuf" ]; then
        PROTOBUF_PROTO="$LOCAL_PROTOBUF_PROTO"
    else
        PROTOBUF_PROTO="$(pkg-config --variable=includedir protobuf 2>/dev/null || echo /usr/include)"
    fi
fi

TABLEAU_PROTO="./third_party/_submodules/tableau/proto"
PROTOCONF_IN="./test/proto"
PROTOCONF_OUT="./test/protoconf"
LOADER_PLUGIN_DIR="./third_party/_submodules/loader/cmd/protoc-gen-go-tableau-loader"
LOADER_OUT="${PROTOCONF_OUT}/tableau"
CHECKER_PLUGIN_DIR="cmd/protoc-gen-go-tableau-checker"
CHECKER_OUT="./test/check"

export PATH="$LOADER_PLUGIN_DIR:$CHECKER_PLUGIN_DIR:$PATH"

# build
cd $LOADER_PLUGIN_DIR && go build && cd -
cd $CHECKER_PLUGIN_DIR && go build && cd -

# remove old generated files
rm -rfv "$PROTOCONF_OUT" "$LOADER_OUT"
mkdir -p "$PROTOCONF_OUT" "$LOADER_OUT" "$CHECKER_OUT"

# generate protoconf files
${PROTOC} \
    --go_out="$PROTOCONF_OUT" \
    --go_opt=paths=source_relative \
    --go-tableau-loader_out="$LOADER_OUT" \
    --go-tableau-loader_opt=paths=source_relative \
    --proto_path="$PROTOBUF_PROTO" \
    --proto_path="$TABLEAU_PROTO" \
    --proto_path="$PROTOCONF_IN" \
    "$PROTOCONF_IN"/*

${PROTOC} \
    --go-tableau-checker_out="$CHECKER_OUT" \
    --go-tableau-checker_opt=paths=source_relative,out="$CHECKER_OUT" \
    --proto_path="$PROTOBUF_PROTO" \
    --proto_path="$TABLEAU_PROTO" \
    --proto_path="$PROTOCONF_IN" \
    "$PROTOCONF_IN"/test_conf.proto # Intended for testing: DO NOT generate "*.check.go" for item_conf.proto
