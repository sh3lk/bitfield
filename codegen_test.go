package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/token"
	"strings"
	"testing"
)

func TestGenerateStructFields(t *testing.T) {
	info := &StructInfo{
		Name: "IPv4Header",
		Fields: []FieldInfo{
			{Name: "Version", Type: Uint8, IsBitField: true, Width: 4, GoType: "uint8"},
			{Name: "IHL", Type: Uint8, IsBitField: true, Width: 4, GoType: "uint8"},
			{Name: "DSCP", Type: Uint8, IsBitField: true, Width: 6, GoType: "uint8"},
			{Name: "ECN", Type: Uint8, IsBitField: true, Width: 2, GoType: "uint8"},
			{Name: "Length", Type: Uint16, IsBitField: true, Width: 16, GoType: "uint16"},
		},
		Layout: Layout{
			Units: []StorageUnit{
				{Index: 0, Type: Uint8, ByteOffset: 0, Fields: []PlacedField{
					{Name: "Version", Type: Uint8, Width: 4, BitOffset: 0},
					{Name: "IHL", Type: Uint8, Width: 4, BitOffset: 4},
				}, UsedBits: 8},
				{Index: 1, Type: Uint8, ByteOffset: 1, Fields: []PlacedField{
					{Name: "DSCP", Type: Uint8, Width: 6, BitOffset: 0},
					{Name: "ECN", Type: Uint8, Width: 2, BitOffset: 6},
				}, UsedBits: 8},
				{Index: 2, Type: Uint16, ByteOffset: 2, Fields: []PlacedField{
					{Name: "Length", Type: Uint16, Width: 16, BitOffset: 0},
				}, UsedBits: 16},
			},
			TotalSize: 4,
			MaxAlign:  2,
		},
	}

	fl := GenerateStructFields(info)
	if len(fl.List) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(fl.List))
	}
	assertEqual(t, "field0.Name", fl.List[0].Names[0].Name, "_bf0")
	assertEqual(t, "field1.Name", fl.List[1].Names[0].Name, "_bf1")
	assertEqual(t, "field2.Name", fl.List[2].Names[0].Name, "_bf2")
}

func TestGenerateStructFieldsWithPadding(t *testing.T) {
	// uint8:3, uint32:5 → padding between unit0 and unit1
	info := &StructInfo{
		Name: "Test",
		Fields: []FieldInfo{
			{Name: "a", Type: Uint8, IsBitField: true, Width: 3, GoType: "uint8"},
			{Name: "b", Type: Uint32, IsBitField: true, Width: 5, GoType: "uint32"},
		},
		Layout: Layout{
			Units: []StorageUnit{
				{Index: 0, Type: Uint8, ByteOffset: 0, Fields: []PlacedField{
					{Name: "a", Type: Uint8, Width: 3, BitOffset: 0},
				}, UsedBits: 3},
				{Index: 1, Type: Uint32, ByteOffset: 4, Fields: []PlacedField{
					{Name: "b", Type: Uint32, Width: 5, BitOffset: 0},
				}, UsedBits: 5},
			},
			TotalSize: 8,
			MaxAlign:  4,
		},
	}

	fl := GenerateStructFields(info)
	if len(fl.List) != 3 {
		t.Fatalf("expected 3 fields (_bf0, _pad0, _bf1), got %d", len(fl.List))
	}
	assertEqual(t, "field0", fl.List[0].Names[0].Name, "_bf0")
	assertEqual(t, "field1", fl.List[1].Names[0].Name, "_pad0")
	assertEqual(t, "field2", fl.List[2].Names[0].Name, "_bf1")
}

func TestFindField(t *testing.T) {
	pkg := &PackageInfo{
		Structs: map[string]*StructInfo{
			"Test": {
				Name: "Test",
				Layout: Layout{
					Units: []StorageUnit{
						{Index: 0, Type: Uint8, Fields: []PlacedField{
							{Name: "A", Type: Uint8, Width: 3, BitOffset: 0},
							{Name: "B", Type: Uint8, Width: 5, BitOffset: 3},
						}},
					},
				},
			},
		},
	}

	unit, field, ok := FindField("A", pkg)
	if !ok {
		t.Fatal("FindField(A) returned false")
	}
	assertEqual(t, "unit.Index", unit.Index, 0)
	assertEqual(t, "field.Name", field.Name, "A")
	assertEqual(t, "field.BitOffset", field.BitOffset, 0)

	unit, field, ok = FindField("B", pkg)
	if !ok {
		t.Fatal("FindField(B) returned false")
	}
	assertEqual(t, "field.Name", field.Name, "B")
	assertEqual(t, "field.BitOffset", field.BitOffset, 3)

	_, _, ok = FindField("C", pkg)
	if ok {
		t.Error("FindField(C) should return false")
	}
}

func TestMakeGetExprUnsigned(t *testing.T) {
	unit := &StorageUnit{
		Index: 0,
		Type:  Uint8,
		Fields: []PlacedField{
			{Name: "Version", Type: Uint8, Width: 4, BitOffset: 0},
			{Name: "IHL", Type: Uint8, Width: 4, BitOffset: 4},
		},
	}

	// Version: uint8(recv._bf0 & 0xf)
	expr := MakeGetExpr(ast.NewIdent("s"), unit, &unit.Fields[0])
	src := formatExpr(t, expr)
	expectContains(t, src, "s._bf0")
	expectContains(t, src, "0xf")

	// IHL: uint8((s._bf0 >> 4) & 0xf)
	expr = MakeGetExpr(ast.NewIdent("s"), unit, &unit.Fields[1])
	src = formatExpr(t, expr)
	expectContains(t, src, ">> 4")
	expectContains(t, src, "0xf")
}

func TestMakeGetExprSigned(t *testing.T) {
	unit := &StorageUnit{
		Index: 0,
		Type:  Uint8,
		Fields: []PlacedField{
			{Name: "val", Type: Int8, Width: 4, BitOffset: 0, Signed: true},
		},
	}

	expr := MakeGetExpr(ast.NewIdent("s"), unit, &unit.Fields[0])
	src := formatExpr(t, expr)
	expectContains(t, src, "int8")
	// Sign extension: << 4 >> 4
	expectContains(t, src, "<< 4")
	expectContains(t, src, ">> 4")
}

func TestMakeGetExprFullWidth(t *testing.T) {
	unit := &StorageUnit{
		Index: 0,
		Type:  Uint16,
		Fields: []PlacedField{
			{Name: "Length", Type: Uint16, Width: 16, BitOffset: 0},
		},
	}

	expr := MakeGetExpr(ast.NewIdent("s"), unit, &unit.Fields[0])
	src := formatExpr(t, expr)
	// Full width, same type → no cast, no mask, just s._bf0
	assertEqual(t, "full-width getter", src, "s._bf0")
}

func TestMakeSetStmtUnsigned(t *testing.T) {
	unit := &StorageUnit{
		Index: 0,
		Type:  Uint8,
		Fields: []PlacedField{
			{Name: "Version", Type: Uint8, Width: 4, BitOffset: 0},
			{Name: "IHL", Type: Uint8, Width: 4, BitOffset: 4},
		},
	}

	// Version setter: s._bf0 = s._bf0&^0xf | uint8(v)&0xf
	stmt := MakeSetStmt(ast.NewIdent("s"), unit, &unit.Fields[0], ast.NewIdent("v"))
	src := formatStmt(t, stmt)
	expectContains(t, src, "s._bf0 =")
	expectContains(t, src, "&^")
	expectContains(t, src, "0xf")

	// IHL setter: s._bf0 = s._bf0&^(0xf<<4) | (uint8(v)&0xf)<<4
	stmt = MakeSetStmt(ast.NewIdent("s"), unit, &unit.Fields[1], ast.NewIdent("v"))
	src = formatStmt(t, stmt)
	expectContains(t, src, "s._bf0 =")
	expectContains(t, src, "<<4")
}

func TestMakeSetStmtFullWidth(t *testing.T) {
	unit := &StorageUnit{
		Index: 0,
		Type:  Uint16,
		Fields: []PlacedField{
			{Name: "Length", Type: Uint16, Width: 16, BitOffset: 0},
		},
	}

	stmt := MakeSetStmt(ast.NewIdent("s"), unit, &unit.Fields[0], ast.NewIdent("v"))
	src := formatStmt(t, stmt)
	expectContains(t, src, "s._bf0 = uint16(v)")
}

// --- helpers ---

func formatExpr(t *testing.T, expr ast.Expr) string {
	t.Helper()
	fset := token.NewFileSet()
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, expr); err != nil {
		t.Fatalf("format.Node: %v", err)
	}
	return buf.String()
}

func formatStmt(t *testing.T, stmt ast.Stmt) string {
	t.Helper()
	// Wrap in a function to format as statement
	fn := &ast.FuncDecl{
		Name: ast.NewIdent("_"),
		Type: &ast.FuncType{},
		Body: &ast.BlockStmt{List: []ast.Stmt{stmt}},
	}
	fset := token.NewFileSet()
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, fn); err != nil {
		t.Fatalf("format.Node: %v", err)
	}
	return buf.String()
}

func expectContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Errorf("expected %q to contain %q", s, sub)
	}
}
