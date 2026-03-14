package main

import (
	"fmt"
	"go/ast"
	"go/token"
)

// GenerateStructFields rewrites an ast.StructType in-place, replacing bitfield
// fields with storage units (_bf0, _bf1, ...) and padding (_pad0, ...).
// Regular fields are preserved. Returns the new field list.
func GenerateStructFields(info *StructInfo) *ast.FieldList {
	layout := &info.Layout
	var fields []*ast.Field

	// Track which regular fields we've emitted (by order in source).
	regularIdx := 0
	unitIdx := 0
	prevEnd := 0 // byte offset after previous member
	padIdx := 0

	// We iterate over units in order and insert regular fields + padding
	// between them based on byte offsets.
	type member struct {
		kind       int // 0=unit, 1=regular
		byteOffset int
		unitIndex  int
		fieldInfo  *FieldInfo
	}

	// Build ordered list of members.
	var members []member
	ri := 0
	ui := 0
	for _, fi := range info.Fields {
		if fi.IsBitField {
			// Find the unit this field belongs to.
			// Multiple bitfields may share the same unit, so only emit
			// the unit when we see its first field.
			if ui < len(layout.Units) {
				unit := &layout.Units[ui]
				if len(unit.Fields) > 0 && unit.Fields[0].Name == fi.Name {
					members = append(members, member{kind: 0, byteOffset: unit.ByteOffset, unitIndex: ui})
					ui++
				}
			}
		} else {
			f := fi // copy
			members = append(members, member{kind: 1, fieldInfo: &f})
			ri++
		}
	}

	_ = regularIdx
	_ = unitIdx
	_ = ri

	for _, m := range members {
		switch m.kind {
		case 0: // storage unit
			unit := &layout.Units[m.unitIndex]
			// Add padding if needed
			if unit.ByteOffset > prevEnd {
				padSize := unit.ByteOffset - prevEnd
				fields = append(fields, makePaddingField(padIdx, padSize))
				padIdx++
			}
			fields = append(fields, makeUnitField(unit))
			prevEnd = unit.ByteOffset + TypeSize(unit.Type)

		case 1: // regular field
			fi := m.fieldInfo
			fields = append(fields, &ast.Field{
				Names: []*ast.Ident{ast.NewIdent(fi.Name)},
				Type:  ast.NewIdent(fi.GoType),
			})
			// Advance prevEnd by field size (approximate; Go handles alignment).
			bt, ok := GoTypeToBackingType(fi.GoType)
			if ok {
				prevEnd += TypeSize(bt)
			}
		}
	}

	return &ast.FieldList{List: fields}
}

func makeUnitField(unit *StorageUnit) *ast.Field {
	name := fmt.Sprintf("_bf%d", unit.Index)
	return &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(name)},
		Type:  ast.NewIdent(BackingTypeToGoName(unit.Type)),
	}
}

func makePaddingField(idx, size int) *ast.Field {
	name := fmt.Sprintf("_pad%d", idx)
	return &ast.Field{
		Names: []*ast.Ident{ast.NewIdent(name)},
		Type: &ast.ArrayType{
			Len: &ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", size)},
			Elt: ast.NewIdent("byte"),
		},
	}
}

// FindField looks up a bitfield by name across all structs in the package.
func FindField(fieldName string, pkg *PackageInfo) (*StorageUnit, *PlacedField, bool) {
	for _, info := range pkg.Structs {
		unit, field, ok := FindFieldInStruct(fieldName, info)
		if ok {
			return unit, field, true
		}
	}
	return nil, nil, false
}

// FindFieldInStruct looks up a bitfield by name within a specific struct.
func FindFieldInStruct(fieldName string, info *StructInfo) (*StorageUnit, *PlacedField, bool) {
	for i := range info.Layout.Units {
		unit := &info.Layout.Units[i]
		for j := range unit.Fields {
			if unit.Fields[j].Name == fieldName {
				return unit, &unit.Fields[j], true
			}
		}
	}
	return nil, nil, false
}

// MakeGetExpr builds an inline expression that reads a bitfield value from recv.
//
// Same type:  (recv._bfN >> shift) & mask
// Signed:     FieldType((recv._bfN >> shift) & mask) << shl >> shl
// Full width: recv._bfN  (or FieldType(recv._bfN) if signed)
func MakeGetExpr(recv ast.Expr, unit *StorageUnit, field *PlacedField) ast.Expr {
	unitName := fmt.Sprintf("_bf%d", unit.Index)

	mask := (1 << field.Width) - 1
	shift := field.BitOffset
	fullWidth := field.Width == TypeSize(unit.Type)*8
	needCast := field.Type != unit.Type // only for signed fields

	// recv._bfN
	sUnit := &ast.SelectorExpr{X: recv, Sel: ast.NewIdent(unitName)}

	// recv._bfN >> shift
	var shifted ast.Expr
	if shift == 0 {
		shifted = sUnit
	} else {
		shifted = &ast.BinaryExpr{X: sUnit, Op: token.SHR, Y: intLit(shift)}
	}

	// (...) & mask
	var masked ast.Expr
	if fullWidth {
		masked = shifted
	} else {
		if shift > 0 {
			shifted = &ast.ParenExpr{X: shifted}
		}
		masked = &ast.BinaryExpr{X: shifted, Op: token.AND, Y: hexLit(mask, "")}
	}

	// Cast only when field type differs from unit type (signed fields).
	var result ast.Expr
	if needCast {
		returnType := BackingTypeToGoName(field.Type)
		result = &ast.CallExpr{Fun: ast.NewIdent(returnType), Args: []ast.Expr{masked}}
	} else {
		result = masked
	}

	if !field.Signed || fullWidth {
		return result
	}

	// Sign extension: FieldType(extract) << shl >> shl
	typeBits := TypeSize(field.Type) * 8
	shl := typeBits - field.Width
	if shl == 0 {
		return result
	}

	return &ast.BinaryExpr{
		X: &ast.BinaryExpr{
			X:  result,
			Op: token.SHL,
			Y:  intLit(shl),
		},
		Op: token.SHR,
		Y:  intLit(shl),
	}
}

// MakeSetStmt builds an inline assignment that writes a bitfield value.
//
// Full width: recv._bfN = UnitType(val)
// Partial:    recv._bfN = recv._bfN&^(mask<<shift) | UnitType(val)&mask<<shift
func MakeSetStmt(recv ast.Expr, unit *StorageUnit, field *PlacedField, val ast.Expr) ast.Stmt {
	unitName := fmt.Sprintf("_bf%d", unit.Index)
	unitType := BackingTypeToGoName(unit.Type)

	mask := (1 << field.Width) - 1
	shift := field.BitOffset
	fullWidth := field.Width == TypeSize(unit.Type)*8

	// LHS: recv._bfN
	lhs := &ast.SelectorExpr{X: recv, Sel: ast.NewIdent(unitName)}

	if fullWidth {
		return &ast.AssignStmt{
			Lhs: []ast.Expr{lhs},
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{castToUnit(unitType, field, val)},
		}
	}

	// RHS recv._bfN (separate node)
	rhsUnit := &ast.SelectorExpr{X: recv, Sel: ast.NewIdent(unitName)}

	// Clear: recv._bfN &^ (mask << shift)
	var clearMask ast.Expr
	if shift == 0 {
		clearMask = hexLit(mask, unitType)
	} else {
		clearMask = &ast.BinaryExpr{X: hexLit(mask, unitType), Op: token.SHL, Y: intLit(shift)}
	}
	cleared := &ast.BinaryExpr{X: rhsUnit, Op: token.AND_NOT, Y: clearMask}

	// Set: build the value part with mask applied.
	// For signed fields, apply mask INSIDE the cast to avoid "constant -X overflows uintN":
	//   uint8(-3 & 0x7) instead of uint8(-3) & 0x7
	// For unsigned fields: uint8(val) & mask
	var valuePart ast.Expr
	if field.Signed {
		// unitType(val & mask) — val & mask is always non-negative
		maskedVal := &ast.BinaryExpr{X: val, Op: token.AND, Y: hexLit(mask, "")}
		valuePart = &ast.CallExpr{Fun: ast.NewIdent(unitType), Args: []ast.Expr{maskedVal}}
	} else {
		castVal := &ast.CallExpr{Fun: ast.NewIdent(unitType), Args: []ast.Expr{val}}
		valuePart = &ast.BinaryExpr{X: castVal, Op: token.AND, Y: hexLit(mask, unitType)}
	}
	if shift > 0 {
		valuePart = &ast.BinaryExpr{X: &ast.ParenExpr{X: valuePart}, Op: token.SHL, Y: intLit(shift)}
	}

	rhs := &ast.BinaryExpr{X: cleared, Op: token.OR, Y: valuePart}

	return &ast.AssignStmt{
		Lhs: []ast.Expr{lhs},
		Tok: token.ASSIGN,
		Rhs: []ast.Expr{rhs},
	}
}

// MakeUnitInitExpr builds an expression that ORs together multiple bitfield values
// for a single storage unit in a composite literal (starting from zero).
//
//	UnitType(v1)&mask1 | (UnitType(v2)&mask2)<<shift2 | ...
func MakeUnitInitExpr(unitType BackingType, fields []*PlacedField, vals []ast.Expr) ast.Expr {
	unitTypeName := BackingTypeToGoName(unitType)
	var result ast.Expr

	for i, field := range fields {
		mask := (1 << field.Width) - 1
		shift := field.BitOffset
		fullWidth := field.Width == TypeSize(unitType)*8

		var part ast.Expr
		if fullWidth {
			part = castToUnit(unitTypeName, field, vals[i])
		} else if field.Signed {
			// For signed: unitType(val & mask) — mask ensures non-negative
			maskedVal := &ast.BinaryExpr{X: vals[i], Op: token.AND, Y: hexLit(mask, "")}
			part = &ast.CallExpr{Fun: ast.NewIdent(unitTypeName), Args: []ast.Expr{maskedVal}}
			if shift > 0 {
				part = &ast.BinaryExpr{X: &ast.ParenExpr{X: part}, Op: token.SHL, Y: intLit(shift)}
			}
		} else {
			castV := &ast.CallExpr{Fun: ast.NewIdent(unitTypeName), Args: []ast.Expr{vals[i]}}
			part = &ast.BinaryExpr{X: castV, Op: token.AND, Y: hexLit(mask, "")}
			if shift > 0 {
				part = &ast.BinaryExpr{X: &ast.ParenExpr{X: part}, Op: token.SHL, Y: intLit(shift)}
			}
		}

		if result == nil {
			result = part
		} else {
			result = &ast.BinaryExpr{
				X:  &ast.ParenExpr{X: result},
				Op: token.OR,
				Y:  &ast.ParenExpr{X: part},
			}
		}
	}
	return result
}

// castToUnit wraps val in a cast to the storage unit type.
// For signed fields, uses a double-cast: UnitType(FieldType(val))
// to avoid "constant -X overflows uintN" errors.
func castToUnit(unitType string, field *PlacedField, val ast.Expr) ast.Expr {
	if field.Signed {
		fieldType := BackingTypeToGoName(field.Type)
		inner := &ast.CallExpr{Fun: ast.NewIdent(fieldType), Args: []ast.Expr{val}}
		return &ast.CallExpr{Fun: ast.NewIdent(unitType), Args: []ast.Expr{inner}}
	}
	return &ast.CallExpr{Fun: ast.NewIdent(unitType), Args: []ast.Expr{val}}
}

func intLit(v int) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("%d", v)}
}

func hexLit(v int, _ string) *ast.BasicLit {
	return &ast.BasicLit{Kind: token.INT, Value: fmt.Sprintf("0x%x", v)}
}
