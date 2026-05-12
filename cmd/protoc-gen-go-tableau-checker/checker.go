package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/bufbuild/protocompile"
	"google.golang.org/protobuf/cmd/protoc-gen-go/internal_gengo"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const checkerProtoPath = "embed/proto/checker.proto"
const checkerProtoName = "checker.proto"
const checkerPBGoName = "checker.pb.go"

// generateCheckerPB compiles checkerProtoPath and delegates to
// internal_gengo.GenerateFile to produce checkerPBGoName.
func generateCheckerPB(gen *protogen.Plugin) error {
	checkerProtoBytes, err := efs.ReadFile(checkerProtoPath)
	if err != nil {
		return fmt.Errorf("read embedded %s: %w", checkerProtoPath, err)
	}

	// Collect non-standard deps from plugin request.
	// google/protobuf/ protos are excluded here because protocompile.WithStandardImports
	// provides its own bundled copies, avoiding "UNVERIFIED" VerificationState errors.
	nonStandardFDs := make(map[string]*descriptorpb.FileDescriptorProto)
	for _, fd := range gen.Request.ProtoFile {
		if !strings.HasPrefix(fd.GetName(), "google/protobuf/") {
			nonStandardFDs[fd.GetName()] = fd
		}
	}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(protocompile.ResolverFunc(func(path string) (protocompile.SearchResult, error) {
			if path == checkerProtoName {
				// Serve checkerProtoPath as source.
				return protocompile.SearchResult{Source: strings.NewReader(string(checkerProtoBytes))}, nil
			}
			if fd, ok := nonStandardFDs[path]; ok {
				// Serve non-standard deps (e.g. tableau/protobuf/tableau.proto).
				return protocompile.SearchResult{Proto: fd}, nil
			}
			return protocompile.SearchResult{}, protoregistry.NotFound
		})),
	}

	linkedFiles, err := compiler.Compile(context.Background(), checkerProtoName)
	if err != nil {
		return fmt.Errorf("compile %s: %w", checkerProtoName, err)
	}

	// Collect checkerProtoName and its transitive deps for the sub-plugin request.
	var allFDs []*descriptorpb.FileDescriptorProto
	seen := make(map[string]bool)
	var collectDeps func(fd protoreflect.FileDescriptor)
	collectDeps = func(fd protoreflect.FileDescriptor) {
		if seen[fd.Path()] {
			return
		}
		seen[fd.Path()] = true
		for i := 0; i < fd.Imports().Len(); i++ {
			collectDeps(fd.Imports().Get(i))
		}
		allFDs = append(allFDs, protodesc.ToFileDescriptorProto(fd))
	}
	collectDeps(linkedFiles.FindFileByPath(checkerProtoName))

	subReq := &pluginpb.CodeGeneratorRequest{
		FileToGenerate:  []string{checkerProtoName},
		ProtoFile:       allFDs,
		Parameter:       proto.String("paths=source_relative"),
		CompilerVersion: gen.Request.CompilerVersion,
	}

	subGen, err := protogen.Options{}.New(subReq)
	if err != nil {
		return fmt.Errorf("create sub-plugin: %w", err)
	}

	for _, f := range subGen.Files {
		if f.Desc.Path() == checkerProtoName {
			gf := internal_gengo.GenerateFile(subGen, f)
			content, err := gf.Content()
			if err != nil {
				return fmt.Errorf("render %s: %w", checkerPBGoName, err)
			}
			out := gen.NewGeneratedFile(checkerPBGoName, "")
			out.P(string(content))
			return nil
		}
	}
	return fmt.Errorf("%s not found in sub-plugin file list", checkerProtoName)
}
