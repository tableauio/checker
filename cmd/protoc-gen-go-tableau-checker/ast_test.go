package main

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAST(t *testing.T) {
	src := `package check

type FooConf struct{}

func (x *FooConf) Check(hub *Hub) error { return nil }
func (x *FooConf) CheckCompatibility(hub, newHub *Hub) error { return nil }
func (x FooConf) ValueRecv() {}
func Helper() {}
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "foo.go", src, 0)
	require.NoError(t, err)

	got := parseAST(file)
	assert.True(t, got[ASTKey{TypeName: "FooConf"}])
	assert.True(t, got[ASTKey{TypeName: "FooConf", FuncName: "Check"}])
	assert.True(t, got[ASTKey{TypeName: "FooConf", FuncName: "CheckCompatibility"}])
	assert.False(t, got[ASTKey{TypeName: "FooConf", FuncName: "ValueRecv"}])
	assert.False(t, got[ASTKey{TypeName: "FooConf", FuncName: "Helper"}])
}

func TestRemoveInitFuncAndTrailingNotes(t *testing.T) {
	src := `package check

import "fmt"

//nolint:gochecknoinits
func init() {
	fmt.Println("register")
}

type FooConf struct{}

func (x *FooConf) Check(hub *Hub) error { return nil }
`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "foo.go", src, parser.ParseComments)
	require.NoError(t, err)

	out, err := removeInitFuncAndTrailingNotes(file, fset)
	require.NoError(t, err)
	assert.NotContains(t, out, "func init()")
	assert.NotContains(t, out, "nolint:gochecknoinits")
	assert.Contains(t, out, "type FooConf struct{}")
	assert.Contains(t, out, "func (x *FooConf) Check")
	// Package clause and remaining decls should still be valid Go.
	_, err = parser.ParseFile(token.NewFileSet(), "foo.go", out, parser.ParseComments)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(strings.TrimSpace(out), "package check"))
}
