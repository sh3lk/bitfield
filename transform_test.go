package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func transformAndFormat(t *testing.T, src string) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	pkg, err := Pass1(fset, []*ast.File{f})
	if err != nil {
		t.Fatalf("Pass1: %v", err)
	}

	if err := Pass2(fset, []*ast.File{f}, pkg); err != nil {
		t.Fatalf("Pass2: %v", err)
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, f); err != nil {
		t.Fatalf("format: %v", err)
	}
	return buf.String()
}

func TestTransformStructRewrite(t *testing.T) {
	src := `package test

type Flags struct {
	A uint8 ` + "`bits:\"3\"`" + `
	B uint8 ` + "`bits:\"5\"`" + `
}
`
	result := transformAndFormat(t, src)

	// Struct should now have _bf0 instead of A, B
	expectContains(t, result, "_bf0")
	expectNotContains(t, result, "A uint8")
	expectNotContains(t, result, "B uint8")

	// No getter/setter methods — they are inlined now
	expectNotContains(t, result, "func (s *Flags)")
}

func TestTransformNoRewriteWithoutBitfields(t *testing.T) {
	src := `package test

type Regular struct {
	X int
	Y int
}

func f() {
	r := Regular{X: 1}
	_ = r.X
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "r.X")
	// Should NOT be rewritten to inline expression
	expectNotContains(t, result, "r._bf")
}

// --- Rewrite tests ---

func TestRewriteRead(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	v := h.Version
	_ = v
}
`
	result := transformAndFormat(t, src)
	// Inlined getter: h._bf0 & 0xf (no cast — same type)
	expectContains(t, result, "h._bf0 & 0xf")
}

func TestRewriteAssign(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	h.Version = 4
}
`
	result := transformAndFormat(t, src)
	// Inlined setter: h._bf0 = h._bf0&^0xf | uint8(4)&0xf
	expectContains(t, result, "h._bf0 = h._bf0&^0xf | uint8(4)&0xf")
}

func TestRewriteCompoundAssign(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	h.Version += 1
}
`
	result := transformAndFormat(t, src)
	// h._bf0 = h._bf0&^0xf | uint8(h._bf0&0xf+1)&0xf
	expectContains(t, result, "h._bf0 = h._bf0&^0xf")
	expectContains(t, result, "h._bf0&0xf+1")
}

func TestRewriteIncrement(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	h.Version++
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "h._bf0 = h._bf0&^0xf")
	expectContains(t, result, "h._bf0&0xf+1")
}

func TestRewriteDecrement(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	h.Version--
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "h._bf0 = h._bf0&^0xf")
	expectContains(t, result, "h._bf0&0xf-1")
}

func TestRewriteCompositeLiteral(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
	IHL     uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	h := H{
		Version: 4,
		IHL:     5,
	}
	_ = h
}
`
	result := transformAndFormat(t, src)
	// Inline set for Version (offset 0) and IHL (offset 4)
	expectContains(t, result, "(uint8(4) & 0xf) | ((uint8(5) & 0xf) << 4)")
}

func TestRewriteAddressOfError(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	_ = &h.Version
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	pkg, err := Pass1(fset, []*ast.File{f})
	if err != nil {
		t.Fatal(err)
	}

	err = Pass2(fset, []*ast.File{f}, pkg)
	if err == nil {
		t.Error("expected error for &h.Version")
	} else {
		expectContains(t, err.Error(), "cannot take address")
	}
}

func TestRewriteInExpression(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	x := h.Version + 1
	_ = x
}
`
	result := transformAndFormat(t, src)
	// h._bf0&0xf + 1 (no cast — same type)
	expectContains(t, result, "h._bf0&0xf + 1")
}

func TestRewriteFunctionArg(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
}

func g(v uint8) {}

func f() {
	var h H
	g(h.Version)
}
`
	result := transformAndFormat(t, src)
	// g(h._bf0 & 0xf)
	expectContains(t, result, "g(h._bf0 & 0xf)")
}

func TestRewriteCompositeLiteralPositional(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
	IHL     uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	h := H{4, 5}
	_ = h
}
`
	result := transformAndFormat(t, src)
	// Same output as keyed
	expectContains(t, result, "_bf0: (uint8(4) & 0xf) | ((uint8(5) & 0xf) << 4)")
	expectNotContains(t, result, "H{4")
}

func TestRewriteCompositeLiteralPositionalMixed(t *testing.T) {
	src := `package test

type M struct {
	X uint8
	A uint8 ` + "`bits:\"3\"`" + `
	B uint8 ` + "`bits:\"5\"`" + `
	Y uint8
}

func f() {
	m := M{0xAA, 5, 12, 0xBB}
	_ = m
}
`
	result := transformAndFormat(t, src)
	// Regular fields become keyed
	expectContains(t, result, "X:")
	expectContains(t, result, "Y:")
	// Bitfields become _bfN
	expectContains(t, result, "_bf0:")
	// No positional elements
	expectNotContains(t, result, "M{0xAA")
}

// --- line preservation ---

func TestLinePreservation(t *testing.T) {
	src := `package test

type H struct {
	Version uint8 ` + "`bits:\"4\"`" + `
	IHL     uint8 ` + "`bits:\"4\"`" + `
	DSCP    uint8 ` + "`bits:\"6\"`" + `
	ECN     uint8 ` + "`bits:\"2\"`" + `
	Length  uint16 ` + "`bits:\"16\"`" + `
}

func f() {
	println("hello")
}
`
	result := transformAndFormat(t, src)

	// Count lines: original has func f() on line 11.
	// After transform, it should still be on the same line.
	lines := strings.Split(result, "\n")
	for i, line := range lines {
		if strings.Contains(line, "func f()") {
			if i+1 != 11 {
				t.Errorf("func f() moved to line %d, want 11\nfull output:\n%s", i+1, result)
			}
			return
		}
	}
	t.Errorf("func f() not found in output:\n%s", result)
}

// --- overflow checks ---

func TestOverflowUnsigned(t *testing.T) {
	src := `package test

type H struct {
	A uint8 ` + "`bits:\"3\"`" + `
}

func f() {
	var h H
	h.A = 8
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Pass1(fset, []*ast.File{f})
	if err != nil {
		t.Fatal(err)
	}
	err = Pass2(fset, []*ast.File{f}, pkg)
	if err == nil {
		t.Error("expected overflow error for h.A = 8 (unsigned 3-bit, max 7)")
	} else {
		expectContains(t, err.Error(), "overflows bitfield")
	}
}

func TestOverflowSigned(t *testing.T) {
	src := `package test

type H struct {
	A int8 ` + "`bits:\"3\"`" + `
}

func f() {
	var h H
	h.A = -5
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Pass1(fset, []*ast.File{f})
	if err != nil {
		t.Fatal(err)
	}
	err = Pass2(fset, []*ast.File{f}, pkg)
	if err == nil {
		t.Error("expected overflow error for h.A = -5 (signed 3-bit, range -4..3)")
	} else {
		expectContains(t, err.Error(), "overflows bitfield")
	}
}

func TestOverflowNegativeUnsigned(t *testing.T) {
	src := `package test

type H struct {
	A uint8 ` + "`bits:\"3\"`" + `
}

func f() {
	var h H
	h.A = -1
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Pass1(fset, []*ast.File{f})
	if err != nil {
		t.Fatal(err)
	}
	err = Pass2(fset, []*ast.File{f}, pkg)
	if err == nil {
		t.Error("expected overflow error for h.A = -1 (unsigned 3-bit)")
	} else {
		expectContains(t, err.Error(), "overflows bitfield")
	}
}

func TestNoOverflowMaxValue(t *testing.T) {
	src := `package test

type H struct {
	A uint8 ` + "`bits:\"3\"`" + `
}

func f() {
	var h H
	h.A = 7
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "_bf0")
}

func TestOverflowCompositeLit(t *testing.T) {
	src := `package test

type H struct {
	A uint8 ` + "`bits:\"3\"`" + `
}

func f() {
	_ = H{A: 8}
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, parser.ParseComments)
	if err != nil {
		t.Fatal(err)
	}
	pkg, err := Pass1(fset, []*ast.File{f})
	if err != nil {
		t.Fatal(err)
	}
	err = Pass2(fset, []*ast.File{f}, pkg)
	if err == nil {
		t.Error("expected overflow error for H{A: 8} (unsigned 3-bit)")
	} else {
		expectContains(t, err.Error(), "overflows bitfield")
	}
}

// --- helpers ---

func expectNotContains(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("expected %q NOT to contain %q", s, sub)
	}
}
