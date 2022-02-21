#!/bin/bash

# set -eux
set -e
set -o pipefail

cd "$(git rev-parse --show-toplevel)"
# PROTOC="./third_party/_submodules/protobuf/src/protoc"
PROTOC="protoc"
PROTOBUF_PROTO="./third_party/_submodules/protobuf/src"
TABLEAU_PROTO="./third_party/_submodules/tableau/proto"
PROTOCONF_IN="./test/proto"
PROTOCONF_OUT="./test/protoconf"
LOADER_PLUGIN_DIR="./third_party/_submodules/loader/cmd/protoc-gen-go-tableau-loader"
CHECKER_PLUGIN_DIR="cmd/protoc-gen-go-tableau-checker"
export PATH="$PATH:$LOADER_PLUGIN_DIR:$CHECKER_PLUGIN_DIR"

# build
cd $LOADER_PLUGIN_DIR && go build && cd -
cd $CHECKER_PLUGIN_DIR && go build && cd -

# remove old generated files
rm -rfv "$PROTOCONF_OUT"
mkdir -p "$PROTOCONF_OUT"

# generate protoconf files
${PROTOC} \
--go_out="$PROTOCONF_OUT" \
--go_opt=paths=source_relative \
--go-tableau-loader_out="$PROTOCONF_OUT" \
--go-tableau-loader_opt=paths=source_relative \
--go-tableau-checker_out="$PROTOCONF_OUT" \
--go-tableau-checker_opt=paths=source_relative \
--proto_path="$PROTOBUF_PROTO" \
--proto_path="$TABLEAU_PROTO" \
--proto_path="$PROTOCONF_IN" \
"$PROTOCONF_IN"/*