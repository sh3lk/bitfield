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

// --- embedding tests ---

func TestEmbeddedFieldPreserved(t *testing.T) {
	src := `package test

type Position struct {
	X int
	Y int
}

type Header struct {
	Position
	flags uint8 ` + "`bits:\"4\"`" + `
	mode  uint8 ` + "`bits:\"4\"`" + `
}
`
	result := transformAndFormat(t, src)
	// Embedded field must survive in the output struct.
	expectContains(t, result, "Position")
	expectContains(t, result, "_bf0")
	expectNotContains(t, result, "flags")
	expectNotContains(t, result, "mode")
}

func TestEmbeddedPromotedRead(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
	count uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
	extra uint32
}

func f() {
	var o Outer
	v := o.flags
	_ = v
}
`
	result := transformAndFormat(t, src)
	// o.flags should be rewritten to access through o.NodeInfo._bf0
	expectContains(t, result, "o.NodeInfo._bf0 & 0xf")
}

func TestEmbeddedPromotedWrite(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
	count uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
	extra uint32
}

func f() {
	var o Outer
	o.flags = 5
}
`
	result := transformAndFormat(t, src)
	// o.flags = 5 should become inline set on o.NodeInfo._bf0
	expectContains(t, result, "o.NodeInfo._bf0 = o.NodeInfo._bf0&^0xf | uint8(5)&0xf")
}

func TestEmbeddedPromotedIncDec(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func f() {
	var o Outer
	o.flags++
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "o.NodeInfo._bf0")
}

func TestEmbeddedExplicitPath(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
	extra uint32
}

func f() {
	var o Outer
	v := o.NodeInfo.flags
	_ = v
}
`
	result := transformAndFormat(t, src)
	// Explicit path o.NodeInfo.flags should also be rewritten.
	expectContains(t, result, "o.NodeInfo._bf0 & 0xf")
}

func TestEmbeddedShadowing(t *testing.T) {
	src := `package test

type Inner struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	Inner
	flags uint32
}

func f() {
	var o Outer
	o.flags = 5
	_ = o
}
`
	result := transformAndFormat(t, src)
	// o.flags refers to Outer.flags (uint32), not Inner.flags (bitfield).
	// It should NOT be rewritten.
	expectContains(t, result, "o.flags = 5")
	expectNotContains(t, result, "o.Inner._bf0")
}

func TestEmbeddedTransitive(t *testing.T) {
	src := `package test

type Base struct {
	x uint8 ` + "`bits:\"4\"`" + `
}

type Mid struct {
	Base
}

type Top struct {
	Mid
}

func f() {
	var t Top
	v := t.x
	_ = v
}
`
	result := transformAndFormat(t, src)
	// t.x should be rewritten through t.Mid.Base._bf0
	expectContains(t, result, "t.Mid.Base._bf0 & 0xf")
}

func TestEmbeddedPointer(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	*NodeInfo
	extra uint32
}

func f() {
	var o Outer
	v := o.flags
	_ = v
}
`
	result := transformAndFormat(t, src)
	// *NodeInfo embedding: field name is still NodeInfo.
	expectContains(t, result, "o.NodeInfo._bf0 & 0xf")
	// The struct should preserve the pointer embedding.
	expectContains(t, result, "*NodeInfo")
}

func TestEmbeddedCompositeLit(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
	count uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
	extra uint32
}

func f() {
	o := Outer{NodeInfo: NodeInfo{flags: 1, count: 2}, extra: 10}
	_ = o
}
`
	result := transformAndFormat(t, src)
	// Inner NodeInfo{...} should be rewritten with _bf0.
	expectContains(t, result, "_bf0:")
	// Outer Outer{...} should preserve NodeInfo: and extra: keys.
	expectContains(t, result, "NodeInfo:")
	expectContains(t, result, "extra:")
}

// --- full-width field optimization ---

func TestFullWidthFieldTreatedAsRegular(t *testing.T) {
	src := `package test

type H struct {
	speed uint8  ` + "`bits:\"8\"`" + `
	flags uint8  ` + "`bits:\"4\"`" + `
	mode  uint8  ` + "`bits:\"4\"`" + `
}

func f() {
	var h H
	h.speed = 42
	v := h.speed
	_ = v
}
`
	result := transformAndFormat(t, src)
	// speed (full-width) should remain a regular field, not packed into _bf.
	expectContains(t, result, "speed")
	expectContains(t, result, "h.speed = 42")
	expectContains(t, result, "h.speed")
	// But flags/mode should be packed.
	expectContains(t, result, "_bf0")
}

func TestFullWidthUint16(t *testing.T) {
	src := `package test

type H struct {
	a uint16 ` + "`bits:\"16\"`" + `
	b uint8  ` + "`bits:\"3\"`" + `
}

func f() {
	var h H
	h.a = 1000
	_ = h.a
}
`
	result := transformAndFormat(t, src)
	// a (16 bits in uint16) is full-width → regular field.
	expectContains(t, result, "h.a = 1000")
	expectContains(t, result, "h.a")
	// b should be packed.
	expectContains(t, result, "_bf0")
}

func TestFullWidthDoesNotCreateBitfieldStruct(t *testing.T) {
	// A struct where ALL fields are full-width should not be treated as a bitfield struct.
	src := `package test

type AllFull struct {
	x uint8  ` + "`bits:\"8\"`" + `
	y uint16 ` + "`bits:\"16\"`" + `
}

func f() {
	var a AllFull
	a.x = 1
	a.y = 2
	_ = a
}
`
	result := transformAndFormat(t, src)
	// No bitfield storage units should appear.
	expectNotContains(t, result, "_bf0")
	// Regular field access preserved.
	expectContains(t, result, "a.x = 1")
	expectContains(t, result, "a.y = 2")
}

// --- cross-struct field name collision (original bug) ---

func TestNoFalseMatchAcrossStructs(t *testing.T) {
	src := `package test

type NodeInfo struct {
	transitionCount uint8 ` + "`bits:\"4\"`" + `
}

type GraphTile struct {
	transitionCount uint32
}

func f() {
	var tile GraphTile
	tile.transitionCount = 100
	_ = tile.transitionCount
}
`
	result := transformAndFormat(t, src)
	// GraphTile.transitionCount is a regular field — must NOT be rewritten.
	expectContains(t, result, "tile.transitionCount = 100")
	expectContains(t, result, "tile.transitionCount")
	expectNotContains(t, result, "tile._bf0")
}

func TestNoFalseMatchUnknownVariable(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

func f(x interface{}) {
	// x is not typed as a bitfield struct — should not be rewritten.
	// (This won't compile in real Go, but tests the rewriter doesn't crash.)
}
`
	// Should not panic or error.
	_ = transformAndFormat(t, src)
}

// --- embedding: compound assignment ---

func TestEmbeddedCompoundAssign(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func f() {
	var o Outer
	o.flags += 1
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "o.NodeInfo._bf0")
	expectContains(t, result, "o.NodeInfo._bf0&0xf+1")
}

// --- embedding: address-of error ---

func TestEmbeddedAddressOfError(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func f() {
	var o Outer
	_ = &o.flags
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
		t.Error("expected error for &o.flags through embedding")
	} else {
		expectContains(t, err.Error(), "cannot take address")
	}
}

// --- embedding: method receiver ---

func TestEmbeddedMethodReceiver(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func (o *Outer) getFlags() uint8 {
	return o.flags
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "o.NodeInfo._bf0 & 0xf")
}

// --- embedding: function parameter ---

func TestEmbeddedFuncParam(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func process(o Outer) uint8 {
	return o.flags
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "o.NodeInfo._bf0 & 0xf")
}

// --- embedding: multiple embedded bitfield structs ---

func TestEmbeddedMultipleBitfieldStructs(t *testing.T) {
	src := `package test

type A struct {
	x uint8 ` + "`bits:\"4\"`" + `
}

type B struct {
	y uint8 ` + "`bits:\"3\"`" + `
}

type C struct {
	A
	B
}

func f() {
	var c C
	vx := c.x
	vy := c.y
	_, _ = vx, vy
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "c.A._bf0 & 0xf")
	expectContains(t, result, "c.B._bf0 & 0x7")
}

// --- embedding: bitfield struct embeds another bitfield struct ---

func TestBitfieldStructEmbedsBitfield(t *testing.T) {
	src := `package test

type Inner struct {
	x uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	Inner
	y uint8 ` + "`bits:\"3\"`" + `
}

func f() {
	var o Outer
	vx := o.x
	vy := o.y
	_, _ = vx, vy
}
`
	result := transformAndFormat(t, src)
	// o.y is a direct bitfield of Outer.
	expectContains(t, result, "o._bf0 & 0x7")
	// o.x goes through embedding: o.Inner._bf0.
	expectContains(t, result, "o.Inner._bf0 & 0xf")
}

// --- embedding: short variable declaration ---

func TestEmbeddedShortDecl(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func f() {
	o := Outer{}
	v := o.flags
	_ = v
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "o.NodeInfo._bf0 & 0xf")
}

// --- embedding: expression context ---

func TestEmbeddedInExpression(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func f() {
	var o Outer
	x := o.flags + 1
	_ = x
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "o.NodeInfo._bf0&0xf + 1")
}

// --- embedding: as function argument ---

func TestEmbeddedAsFuncArg(t *testing.T) {
	src := `package test

type NodeInfo struct {
	flags uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	NodeInfo
}

func g(v uint8) {}

func f() {
	var o Outer
	g(o.flags)
}
`
	result := transformAndFormat(t, src)
	expectContains(t, result, "g(o.NodeInfo._bf0 & 0xf)")
}

// --- embedding: shadowing in bitfield struct ---

func TestShadowingInBitfieldStruct(t *testing.T) {
	src := `package test

type Inner struct {
	x uint8 ` + "`bits:\"4\"`" + `
}

type Outer struct {
	Inner
	x uint8 ` + "`bits:\"3\"`" + `
}

func f() {
	var o Outer
	o.x = 5
}
`
	result := transformAndFormat(t, src)
	// o.x should resolve to Outer's own bitfield (3-bit), not Inner's (4-bit).
	// Mask 0x7 = 3 bits, not 0xf = 4 bits.
	expectContains(t, result, "0x7")
	expectNotContains(t, result, "o.Inner._bf0")
}

// --- embedding: embedded field in positional composite literal ---

func TestEmbeddedPositionalCompositeLit(t *testing.T) {
	src := `package test

type Pos struct {
	X int
}

type H struct {
	Pos
	flags uint8 ` + "`bits:\"4\"`" + `
	mode  uint8 ` + "`bits:\"4\"`" + `
}

func f() {
	h := H{Pos{1}, 2, 3}
	_ = h
}
`
	result := transformAndFormat(t, src)
	// Embedded field preserved as keyed.
	expectContains(t, result, "Pos:")
	// Bitfields packed.
	expectContains(t, result, "_bf0:")
}

// --- helpers ---

func expectNotContains(t *testing.T, s, sub string) {
	t.Helper()
	if strings.Contains(s, sub) {
		t.Errorf("expected %q NOT to contain %q", s, sub)
	}
}
