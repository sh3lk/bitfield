package basic

import (
	"testing"
	"unsafe"
)

type SignedFields struct {
	Val int8 `bits:"4"`
}

func TestSignedExtension(t *testing.T) {
	var s SignedFields
	// Set -3 (4-bit signed: 1101 → sign-extended to int8: 11111101 = -3)
	s.Val = -3
	if s.Val != -3 {
		t.Errorf("expected -3, got %d", s.Val)
	}

	// Set 7 (max positive for 4-bit signed)
	s.Val = 7
	if s.Val != 7 {
		t.Errorf("expected 7, got %d", s.Val)
	}

	// Set -8 (min for 4-bit signed)
	s.Val = -8
	if s.Val != -8 {
		t.Errorf("expected -8, got %d", s.Val)
	}
}

type SizeCheck struct {
	A uint8 `bits:"3"`
	B uint8 `bits:"5"`
}

func TestStructSize(t *testing.T) {
	// {uint8:3, uint8:5} → 1 byte
	size := unsafe.Sizeof(SizeCheck{})
	if size != 1 {
		t.Errorf("expected sizeof=1, got %d", size)
	}
}

type IPv4 struct {
	Version uint8  `bits:"4"`
	IHL     uint8  `bits:"4"`
	DSCP    uint8  `bits:"6"`
	ECN     uint8  `bits:"2"`
	Length  uint16 `bits:"16"`
}

func TestIPv4Size(t *testing.T) {
	size := unsafe.Sizeof(IPv4{})
	if size != 4 {
		t.Errorf("expected sizeof=4, got %d", size)
	}
}

func TestIPv4FieldsIsolation(t *testing.T) {
	var h IPv4
	h.Version = 4
	h.IHL = 5
	h.DSCP = 46
	h.ECN = 3
	h.Length = 1500

	if h.Version != 4 {
		t.Errorf("Version: expected 4, got %d", h.Version)
	}
	if h.IHL != 5 {
		t.Errorf("IHL: expected 5, got %d", h.IHL)
	}
	if h.DSCP != 46 {
		t.Errorf("DSCP: expected 46, got %d", h.DSCP)
	}
	if h.ECN != 3 {
		t.Errorf("ECN: expected 3, got %d", h.ECN)
	}
	if h.Length != 1500 {
		t.Errorf("Length: expected 1500, got %d", h.Length)
	}
}
