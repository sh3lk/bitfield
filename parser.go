package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"reflect"
	"strconv"
	"strings"
)

// BitFieldTag holds the parsed content of a `bits:"N"` struct tag.
type BitFieldTag struct {
	Width int
}

// ParseBitsTag extracts the bit width from a struct tag string like `bits:"4"`.
// Returns nil if no bits tag is present.
func ParseBitsTag(tag string) (*BitFieldTag, error) {
	if tag == "" {
		return nil, nil
	}
	st := reflect.StructTag(tag)
	val, ok := st.Lookup("bits")
	if !ok {
		return nil, nil
	}
	val = strings.TrimSpace(val)
	width, err := strconv.Atoi(val)
	if err != nil {
		return nil, fmt.Errorf("invalid bits tag value %q: %w", val, err)
	}
	return &BitFieldTag{Width: width}, nil
}

// GoTypeToBackingType converts a Go type name to a BackingType.
func GoTypeToBackingType(typeName string) (BackingType, bool) {
	switch typeName {
	case "uint8":
		return Uint8, true
	case "uint16":
		return Uint16, true
	case "uint32":
		return Uint32, true
	case "uint64":
		return Uint64, true
	case "int8":
		return Int8, true
	case "int16":
		return Int16, true
	case "int32":
		return Int32, true
	case "int64":
		return Int64, true
	default:
		return 0, false
	}
}

// BackingTypeToGoName returns the Go type name for a BackingType.
func BackingTypeToGoName(t BackingType) string {
	return t.String()
}

// StructInfo holds parsed information about a struct with bitfields.
type StructInfo struct {
	Name   string
	Fields []FieldInfo
	Layout Layout
}

// FieldInfo holds parsed information about a single struct field.
type FieldInfo struct {
	Name       string
	Type       BackingType
	IsBitField bool
	Width      int // bit width, only for bitfields
	GoType     string
}

// ParseStructType extracts bitfield information from an ast.StructType.
// Returns nil if the struct has no bitfields.
func ParseStructType(fset *token.FileSet, structName string, st *ast.StructType) (*StructInfo, error) {
	if st.Fields == nil || len(st.Fields.List) == 0 {
		return nil, nil
	}

	hasBitField := false
	var fieldInfos []FieldInfo
	var descriptors []FieldDescriptor

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			// Embedded field — skip (not supported as bitfield)
			continue
		}

		goTypeName := typeExprToString(field.Type)

		// Parse tag
		var tag string
		if field.Tag != nil {
			// Remove backticks from raw string literal
			tag = field.Tag.Value
			if len(tag) >= 2 {
				tag = tag[1 : len(tag)-1]
			}
		}

		bt, err := ParseBitsTag(tag)
		if err != nil {
			pos := fset.Position(field.Pos())
			return nil, fmt.Errorf("%s:%d: %w", pos.Filename, pos.Line, err)
		}

		for _, name := range field.Names {
			fi := FieldInfo{
				Name:   name.Name,
				GoType: goTypeName,
			}

			if bt != nil {
				// Bitfield
				backingType, ok := GoTypeToBackingType(goTypeName)
				if !ok {
					pos := fset.Position(field.Pos())
					return nil, fmt.Errorf("%s:%d: field %q: unsupported type %q for bitfield",
						pos.Filename, pos.Line, name.Name, goTypeName)
				}
				fi.Type = backingType
				fi.IsBitField = true
				fi.Width = bt.Width
				hasBitField = true

				descriptors = append(descriptors, FieldDescriptor{
					Name:       name.Name,
					Type:       backingType,
					Width:      bt.Width,
					IsBitField: true,
				})
			} else {
				// Regular field
				backingType, ok := GoTypeToBackingType(goTypeName)
				if ok {
					fi.Type = backingType
					descriptors = append(descriptors, FieldDescriptor{
						Name:       name.Name,
						Type:       backingType,
						IsBitField: false,
						Size:       TypeSize(backingType),
						Align:      TypeAlign(backingType),
					})
				} else {
					// Non-integer type — we can't compute exact size,
					// but we still track it to break bitfield groups.
					// For now, treat as opaque and reset unit tracking.
					descriptors = append(descriptors, FieldDescriptor{
						Name:       name.Name,
						IsBitField: false,
						Size:       0, // unknown
						Align:      1,
					})
				}
			}

			fieldInfos = append(fieldInfos, fi)
		}
	}

	if !hasBitField {
		return nil, nil
	}

	layout, err := ComputeLayout(descriptors)
	if err != nil {
		return nil, err
	}

	return &StructInfo{
		Name:   structName,
		Fields: fieldInfos,
		Layout: layout,
	}, nil
}

// typeExprToString converts a type AST expression to its string representation.
func typeExprToString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeExprToString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeExprToString(t.X)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + typeExprToString(t.Elt)
		}
		return "[" + typeExprToString(t.Len) + "]" + typeExprToString(t.Elt)
	default:
		return ""
	}
}
