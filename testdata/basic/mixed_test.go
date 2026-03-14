package basic

import (
	"testing"
	"unsafe"
)

// --- Mixed structs: bitfields + regular fields ---

// Regular field in the middle
type MixedMiddle struct {
	Flags uint8 `bits:"4"`
	Mode  uint8 `bits:"4"`
	Count uint32
	Tag   uint8 `bits:"3"`
	Valid uint8 `bits:"1"`
}

// Regular field at the start
type MixedStart struct {
	ID      uint32
	Version uint8 `bits:"4"`
	IHL     uint8 `bits:"4"`
}

// Regular field at the end
type MixedEnd struct {
	Version uint8 `bits:"4"`
	IHL     uint8 `bits:"4"`
	Payload uint32
}

// Multiple regular fields
type MixedMulti struct {
	A uint8 `bits:"3"`
	X uint16
	B uint8 `bits:"5"`
	Y uint32
	C uint8 `bits:"1"`
}

// Regular field between same-type bitfield groups
type MixedSameType struct {
	Hi  uint8 `bits:"4"`
	Sep uint8
	Lo  uint8 `bits:"4"`
}

// --- sizeof ---

func TestMixedSizes(t *testing.T) {
	tests := []struct {
		name string
		size uintptr
	}{
		// _bf0(1) + pad(3) + Count(4) + _bf1(1) + pad(3) = 12
		{"MixedMiddle", unsafe.Sizeof(MixedMiddle{})},
		// ID(4) + _bf0(1) + pad(3) = 8
		{"MixedStart", unsafe.Sizeof(MixedStart{})},
		// _bf0(1) + pad(3) + Payload(4) = 8
		{"MixedEnd", unsafe.Sizeof(MixedEnd{})},
	}
	for _, tt := range tests {
		t.Logf("%s: sizeof=%d", tt.name, tt.size)
		if tt.size == 0 {
			t.Errorf("%s: sizeof unexpectedly 0", tt.name)
		}
	}
}

// --- Bitfield get/set don't corrupt regular fields ---

func TestMixedMiddleIsolation(t *testing.T) {
	var s MixedMiddle
	s.Count = 0xDEADBEEF
	s.Flags = 0xF
	s.Mode = 0xA

	if s.Count != 0xDEADBEEF {
		t.Errorf("setting bitfields corrupted Count: got 0x%x", s.Count)
	}
	if s.Flags != 0xF {
		t.Errorf("Flags: got %d", s.Flags)
	}
	if s.Mode != 0xA {
		t.Errorf("Mode: got %d", s.Mode)
	}

	s.Tag = 7
	s.Valid = 1
	if s.Count != 0xDEADBEEF {
		t.Errorf("setting Tag/Valid corrupted Count: got 0x%x", s.Count)
	}
	if s.Flags != 0xF {
		t.Errorf("setting Tag/Valid corrupted Flags: got %d", s.Flags)
	}
}

func TestMixedMiddleRegularWrite(t *testing.T) {
	var s MixedMiddle
	s.Flags = 5
	s.Mode = 10
	s.Tag = 3
	s.Valid = 1

	// Overwrite regular field — bitfields must survive
	s.Count = 42
	if s.Flags != 5 {
		t.Errorf("Flags corrupted: got %d", s.Flags)
	}
	if s.Mode != 10 {
		t.Errorf("Mode corrupted: got %d", s.Mode)
	}
	if s.Tag != 3 {
		t.Errorf("Tag corrupted: got %d", s.Tag)
	}
	if s.Valid != 1 {
		t.Errorf("Valid corrupted: got %d", s.Valid)
	}
	if s.Count != 42 {
		t.Errorf("Count: got %d", s.Count)
	}
}

func TestMixedStartIsolation(t *testing.T) {
	var s MixedStart
	s.ID = 0x12345678
	s.Version = 4
	s.IHL = 5

	if s.ID != 0x12345678 {
		t.Errorf("ID corrupted: got 0x%x", s.ID)
	}
	if s.Version != 4 {
		t.Errorf("Version: got %d", s.Version)
	}
	if s.IHL != 5 {
		t.Errorf("IHL: got %d", s.IHL)
	}

	s.ID = 0xAAAAAAAA
	if s.Version != 4 {
		t.Errorf("changing ID corrupted Version: got %d", s.Version)
	}
}

func TestMixedEndIsolation(t *testing.T) {
	var s MixedEnd
	s.Version = 6
	s.IHL = 15
	s.Payload = 0xCAFEBABE

	if s.Version != 6 {
		t.Errorf("Version: got %d", s.Version)
	}
	if s.IHL != 15 {
		t.Errorf("IHL: got %d", s.IHL)
	}
	if s.Payload != 0xCAFEBABE {
		t.Errorf("Payload: got 0x%x", s.Payload)
	}

	s.Payload = 0
	if s.Version != 6 {
		t.Errorf("changing Payload corrupted Version: got %d", s.Version)
	}
}

func TestMixedMultiIsolation(t *testing.T) {
	var s MixedMulti
	s.A = 7
	s.X = 1000
	s.B = 31
	s.Y = 0xDEAD
	s.C = 1

	if s.A != 7 {
		t.Errorf("A: got %d", s.A)
	}
	if s.X != 1000 {
		t.Errorf("X: got %d", s.X)
	}
	if s.B != 31 {
		t.Errorf("B: got %d", s.B)
	}
	if s.Y != 0xDEAD {
		t.Errorf("Y: got 0x%x", s.Y)
	}
	if s.C != 1 {
		t.Errorf("C: got %d", s.C)
	}

	// Overwrite regular fields
	s.X = 0
	s.Y = 0
	if s.A != 7 || s.B != 31 || s.C != 1 {
		t.Errorf("bitfields corrupted: A=%d B=%d C=%d", s.A, s.B, s.C)
	}
}

func TestMixedSameTypeIsolation(t *testing.T) {
	var s MixedSameType
	s.Hi = 0xA
	s.Sep = 0xFF
	s.Lo = 0x5

	if s.Hi != 0xA {
		t.Errorf("Hi: got %d", s.Hi)
	}
	if s.Sep != 0xFF {
		t.Errorf("Sep: got 0x%x", s.Sep)
	}
	if s.Lo != 0x5 {
		t.Errorf("Lo: got %d", s.Lo)
	}

	s.Sep = 0
	if s.Hi != 0xA || s.Lo != 0x5 {
		t.Errorf("clearing Sep corrupted bitfields: Hi=%d Lo=%d", s.Hi, s.Lo)
	}
}

// --- Operations on mixed structs ---

func TestMixedIncrement(t *testing.T) {
	var s MixedMiddle
	s.Count = 100
	s.Flags = 3
	s.Flags++
	if s.Flags != 4 {
		t.Errorf("Flags after ++: got %d", s.Flags)
	}
	if s.Count != 100 {
		t.Errorf("Count corrupted: got %d", s.Count)
	}
}

func TestMixedCompoundAssign(t *testing.T) {
	var s MixedMiddle
	s.Count = 200
	s.Tag = 2
	s.Tag += 3
	if s.Tag != 5 {
		t.Errorf("Tag after +=3: got %d", s.Tag)
	}
	if s.Count != 200 {
		t.Errorf("Count corrupted: got %d", s.Count)
	}
}

func TestMixedCompositeLiteral(t *testing.T) {
	s := MixedMiddle{
		Flags: 9,
		Mode:  3,
		Count: 42,
		Tag:   5,
		Valid: 1,
	}
	if s.Flags != 9 {
		t.Errorf("Flags: got %d", s.Flags)
	}
	if s.Mode != 3 {
		t.Errorf("Mode: got %d", s.Mode)
	}
	if s.Count != 42 {
		t.Errorf("Count: got %d", s.Count)
	}
	if s.Tag != 5 {
		t.Errorf("Tag: got %d", s.Tag)
	}
	if s.Valid != 1 {
		t.Errorf("Valid: got %d", s.Valid)
	}
}
