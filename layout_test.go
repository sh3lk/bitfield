package main

import "testing"

func TestLayoutSingleUnit(t *testing.T) {
	// { uint8:3, uint8:5 } → 1 byte, one unit
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint8, Width: 3, IsBitField: true},
		{Name: "b", Type: Uint8, Width: 5, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 1)
	assertEqual(t, "len(Units)", len(layout.Units), 1)
	assertEqual(t, "UsedBits", layout.Units[0].UsedBits, 8)
	assertEqual(t, "a.BitOffset", layout.Units[0].Fields[0].BitOffset, 0)
	assertEqual(t, "b.BitOffset", layout.Units[0].Fields[1].BitOffset, 3)
}

func TestLayoutOverflow(t *testing.T) {
	// { uint8:3, uint8:6 } → 2 bytes, overflow to second unit
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint8, Width: 3, IsBitField: true},
		{Name: "b", Type: Uint8, Width: 6, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 2)
	assertEqual(t, "len(Units)", len(layout.Units), 2)
	assertEqual(t, "unit0.ByteOffset", layout.Units[0].ByteOffset, 0)
	assertEqual(t, "unit1.ByteOffset", layout.Units[1].ByteOffset, 1)
	assertEqual(t, "a.BitOffset", layout.Units[0].Fields[0].BitOffset, 0)
	assertEqual(t, "b.BitOffset", layout.Units[1].Fields[0].BitOffset, 0)
}

func TestLayoutTypeChangeAlign2(t *testing.T) {
	// { uint8:3, uint16:5 } → 4 bytes (type change + align to 2)
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint8, Width: 3, IsBitField: true},
		{Name: "b", Type: Uint16, Width: 5, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 4)
	assertEqual(t, "len(Units)", len(layout.Units), 2)
	assertEqual(t, "unit0.ByteOffset", layout.Units[0].ByteOffset, 0)
	assertEqual(t, "unit0.Type", int(layout.Units[0].Type), int(Uint8))
	assertEqual(t, "unit1.ByteOffset", layout.Units[1].ByteOffset, 2)
	assertEqual(t, "unit1.Type", int(layout.Units[1].Type), int(Uint16))
}

func TestLayoutTypeChangeAlign4(t *testing.T) {
	// { uint8:3, uint32:5 } → 8 bytes (leading padding to align=4)
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint8, Width: 3, IsBitField: true},
		{Name: "b", Type: Uint32, Width: 5, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 8)
	assertEqual(t, "len(Units)", len(layout.Units), 2)
	assertEqual(t, "unit0.ByteOffset", layout.Units[0].ByteOffset, 0)
	assertEqual(t, "unit1.ByteOffset", layout.Units[1].ByteOffset, 4)
}

func TestLayoutOverflowSameType(t *testing.T) {
	// { uint32:16, uint32:20 } → 8 bytes (overflow → 2 units)
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint32, Width: 16, IsBitField: true},
		{Name: "b", Type: Uint32, Width: 20, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 8)
	assertEqual(t, "len(Units)", len(layout.Units), 2)
	assertEqual(t, "unit0.ByteOffset", layout.Units[0].ByteOffset, 0)
	assertEqual(t, "unit1.ByteOffset", layout.Units[1].ByteOffset, 4)
	assertEqual(t, "a.BitOffset", layout.Units[0].Fields[0].BitOffset, 0)
	assertEqual(t, "b.BitOffset", layout.Units[1].Fields[0].BitOffset, 0)
}

func TestLayoutRegularFieldBreaksGroup(t *testing.T) {
	// { uint8:4, regular int32, uint8:3 } → 8 bytes
	// Regular field breaks bitfield group and contributes to maxAlign.
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint8, Width: 4, IsBitField: true},
		{Name: "b", Type: Int32, IsBitField: false, Size: 4, Align: 4},
		{Name: "c", Type: Uint8, Width: 3, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 8)
	assertEqual(t, "len(Units)", len(layout.Units), 2)
	assertEqual(t, "unit0.Fields[0].Name", layout.Units[0].Fields[0].Name, "a")
	assertEqual(t, "unit1.Fields[0].Name", layout.Units[1].Fields[0].Name, "c")
}

func TestLayoutIPv4Header(t *testing.T) {
	// IPv4Header from PRD
	fields := []FieldDescriptor{
		{Name: "Version", Type: Uint8, Width: 4, IsBitField: true},
		{Name: "IHL", Type: Uint8, Width: 4, IsBitField: true},
		{Name: "DSCP", Type: Uint8, Width: 6, IsBitField: true},
		{Name: "ECN", Type: Uint8, Width: 2, IsBitField: true},
		{Name: "Length", Type: Uint16, Width: 16, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 4)
	assertEqual(t, "len(Units)", len(layout.Units), 3)

	// unit0: Version(4) + IHL(4) = 8 bits in uint8
	assertEqual(t, "unit0.Type", int(layout.Units[0].Type), int(Uint8))
	assertEqual(t, "unit0.ByteOffset", layout.Units[0].ByteOffset, 0)
	assertEqual(t, "unit0.UsedBits", layout.Units[0].UsedBits, 8)

	// unit1: DSCP(6) + ECN(2) = 8 bits in uint8
	assertEqual(t, "unit1.Type", int(layout.Units[1].Type), int(Uint8))
	assertEqual(t, "unit1.ByteOffset", layout.Units[1].ByteOffset, 1)
	assertEqual(t, "unit1.UsedBits", layout.Units[1].UsedBits, 8)

	// unit2: Length(16) in uint16
	assertEqual(t, "unit2.Type", int(layout.Units[2].Type), int(Uint16))
	assertEqual(t, "unit2.ByteOffset", layout.Units[2].ByteOffset, 2)
	assertEqual(t, "unit2.UsedBits", layout.Units[2].UsedBits, 16)
}

func TestLayoutSignedField(t *testing.T) {
	// Signed bitfield: backing unit is unsigned, field is marked signed
	fields := []FieldDescriptor{
		{Name: "val", Type: Int8, Width: 4, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 1)
	assertEqual(t, "unit.Type", int(layout.Units[0].Type), int(Uint8))
	if !layout.Units[0].Fields[0].Signed {
		t.Error("expected field to be marked signed")
	}
}

func TestLayoutTrailingPadding(t *testing.T) {
	// { uint8:3, uint32:1 } → uint8 unit (1 byte) + padding (3 bytes) + uint32 unit (4 bytes) = 8
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint8, Width: 3, IsBitField: true},
		{Name: "b", Type: Uint32, Width: 1, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 8)
	assertEqual(t, "MaxAlign", layout.MaxAlign, 4)
}

func TestLayoutEmptyStruct(t *testing.T) {
	layout, err := ComputeLayout(nil)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 0)
	assertEqual(t, "len(Units)", len(layout.Units), 0)
}

func TestLayoutUint64(t *testing.T) {
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint64, Width: 33, IsBitField: true},
		{Name: "b", Type: Uint64, Width: 31, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	assertEqual(t, "TotalSize", layout.TotalSize, 8)
	assertEqual(t, "len(Units)", len(layout.Units), 1)
	assertEqual(t, "UsedBits", layout.Units[0].UsedBits, 64)
}

func TestLayoutMixedSignedUnsignedSameSize(t *testing.T) {
	// int8 and uint8 have different BackingTypes → new unit even though same size
	fields := []FieldDescriptor{
		{Name: "a", Type: Uint8, Width: 3, IsBitField: true},
		{Name: "b", Type: Int8, Width: 3, IsBitField: true},
	}
	layout, err := ComputeLayout(fields)
	if err != nil {
		t.Fatal(err)
	}
	// UnsignedOf(Int8) = Uint8, so they share the same backing type → same unit
	assertEqual(t, "len(Units)", len(layout.Units), 1)
	assertEqual(t, "TotalSize", layout.TotalSize, 1)
	if !layout.Units[0].Fields[1].Signed {
		t.Error("expected field b to be marked signed")
	}
}

// --- Validation tests ---

func TestValidateWidthZero(t *testing.T) {
	fields := []FieldDescriptor{
		{Name: "bad", Type: Uint8, Width: 0, IsBitField: true},
	}
	_, err := ComputeLayout(fields)
	if err == nil {
		t.Error("expected error for width=0")
	}
}

func TestValidateWidthExceedsType(t *testing.T) {
	fields := []FieldDescriptor{
		{Name: "bad", Type: Uint8, Width: 9, IsBitField: true},
	}
	_, err := ComputeLayout(fields)
	if err == nil {
		t.Error("expected error for width > 8")
	}
}

func TestValidateWidthExactlyTypeSize(t *testing.T) {
	// Width == sizeof(type)*8 is valid
	fields := []FieldDescriptor{
		{Name: "ok", Type: Uint16, Width: 16, IsBitField: true},
	}
	_, err := ComputeLayout(fields)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Helpers ---

func assertEqual[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got %v, want %v", name, got, want)
	}
}
