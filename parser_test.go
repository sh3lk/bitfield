package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

func TestParseBitsTag(t *testing.T) {
	tests := []struct {
		tag     string
		want    *BitFieldTag
		wantErr bool
	}{
		{``, nil, false},
		{`json:"name"`, nil, false},
		{`bits:"4"`, &BitFieldTag{Width: 4}, false},
		{`bits:"16"`, &BitFieldTag{Width: 16}, false},
		{`json:"name" bits:"8"`, &BitFieldTag{Width: 8}, false},
		{`bits:"abc"`, nil, true},
	}
	for _, tt := range tests {
		got, err := ParseBitsTag(tt.tag)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseBitsTag(%q): expected error", tt.tag)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseBitsTag(%q): unexpected error: %v", tt.tag, err)
			continue
		}
		if tt.want == nil && got != nil {
			t.Errorf("ParseBitsTag(%q): expected nil, got %+v", tt.tag, got)
		}
		if tt.want != nil {
			if got == nil {
				t.Errorf("ParseBitsTag(%q): expected %+v, got nil", tt.tag, tt.want)
			} else if got.Width != tt.want.Width {
				t.Errorf("ParseBitsTag(%q): width got %d, want %d", tt.tag, got.Width, tt.want.Width)
			}
		}
	}
}

func TestGoTypeToBackingType(t *testing.T) {
	tests := []struct {
		name string
		want BackingType
		ok   bool
	}{
		{"uint8", Uint8, true},
		{"uint16", Uint16, true},
		{"uint32", Uint32, true},
		{"uint64", Uint64, true},
		{"int8", Int8, true},
		{"int16", Int16, true},
		{"int32", Int32, true},
		{"int64", Int64, true},
		{"string", 0, false},
		{"float64", 0, false},
	}
	for _, tt := range tests {
		got, ok := GoTypeToBackingType(tt.name)
		if ok != tt.ok {
			t.Errorf("GoTypeToBackingType(%q): ok=%v, want %v", tt.name, ok, tt.ok)
		}
		if ok && got != tt.want {
			t.Errorf("GoTypeToBackingType(%q): got %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestParseStructType(t *testing.T) {
	src := `package test

type IPv4Header struct {
	Version uint8  ` + "`bits:\"4\"`" + `
	IHL     uint8  ` + "`bits:\"4\"`" + `
	DSCP    uint8  ` + "`bits:\"6\"`" + `
	ECN     uint8  ` + "`bits:\"2\"`" + `
	Length  uint16 ` + "`bits:\"16\"`" + `
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	var info *StructInfo
	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			info, err = ParseStructType(fset, ts.Name.Name, st)
			if err != nil {
				t.Fatal(err)
			}
		}
	}

	if info == nil {
		t.Fatal("expected StructInfo, got nil")
	}
	assertEqual(t, "Name", info.Name, "IPv4Header")
	assertEqual(t, "len(Fields)", len(info.Fields), 5)
	assertEqual(t, "TotalSize", info.Layout.TotalSize, 4)
	assertEqual(t, "len(Units)", len(info.Layout.Units), 3)

	// Check fields
	assertEqual(t, "Fields[0].Name", info.Fields[0].Name, "Version")
	assertEqual(t, "Fields[0].Width", info.Fields[0].Width, 4)
	if !info.Fields[0].IsBitField {
		t.Error("Fields[0] should be bitfield")
	}
}

func TestParseStructNoBitfields(t *testing.T) {
	src := `package test

type Regular struct {
	X int
	Y int
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			info, err := ParseStructType(fset, ts.Name.Name, st)
			if err != nil {
				t.Fatal(err)
			}
			if info != nil {
				t.Error("expected nil for struct without bitfields")
			}
		}
	}
}

func TestParseStructMixed(t *testing.T) {
	src := `package test

type Mixed struct {
	Flags uint8  ` + "`bits:\"4\"`" + `
	Mode  uint8  ` + "`bits:\"4\"`" + `
	Name  string
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			info, err := ParseStructType(fset, ts.Name.Name, st)
			if err != nil {
				t.Fatal(err)
			}
			if info == nil {
				t.Fatal("expected StructInfo for mixed struct")
			}
			assertEqual(t, "len(Fields)", len(info.Fields), 3)
			if info.Fields[2].IsBitField {
				t.Error("Name field should not be bitfield")
			}
		}
	}
}

func TestParseStructInvalidTag(t *testing.T) {
	src := `package test

type Bad struct {
	X uint8 ` + "`bits:\"abc\"`" + `
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			_, err := ParseStructType(fset, ts.Name.Name, st)
			if err == nil {
				t.Error("expected error for invalid bits tag")
			}
		}
	}
}

func TestParseStructUnsupportedType(t *testing.T) {
	src := `package test

type Bad struct {
	X float64 ` + "`bits:\"4\"`" + `
}
`
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}

	for _, decl := range f.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}
			_, err := ParseStructType(fset, ts.Name.Name, st)
			if err == nil {
				t.Error("expected error for unsupported bitfield type")
			}
		}
	}
}
