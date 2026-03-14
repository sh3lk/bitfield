package basic

import (
	"testing"
	"unsafe"
)

// --- Structures for testing ---

type TwoFields8 struct {
	A uint8 `bits:"3"`
	B uint8 `bits:"5"`
}

type Overflow8 struct {
	A uint8 `bits:"3"`
	B uint8 `bits:"6"`
}

type CrossType struct {
	A uint8  `bits:"3"`
	B uint16 `bits:"12"`
}

type FullWidth struct {
	A uint8  `bits:"8"`
	B uint16 `bits:"16"`
	C uint32 `bits:"32"`
}

type FourNibbles struct {
	A uint8 `bits:"2"`
	B uint8 `bits:"2"`
	C uint8 `bits:"2"`
	D uint8 `bits:"2"`
}

type Wide32 struct {
	A uint32 `bits:"16"`
	B uint32 `bits:"16"`
}

type Wide64 struct {
	A uint64 `bits:"33"`
	B uint64 `bits:"31"`
}

type SignedSmall struct {
	A int8 `bits:"3"`
	B int8 `bits:"5"`
}

type Signed16 struct {
	Val int16 `bits:"10"`
}

type Signed32 struct {
	Val int32 `bits:"20"`
}

type SingleBit struct {
	Flag uint8 `bits:"1"`
}

type SingleBitSigned struct {
	Flag int8 `bits:"1"`
}

// --- Size tests ---

func TestSizes(t *testing.T) {
	tests := []struct {
		name string
		size uintptr
		want uintptr
	}{
		{"TwoFields8", unsafe.Sizeof(TwoFields8{}), 1},
		{"Overflow8", unsafe.Sizeof(Overflow8{}), 2},
		{"CrossType", unsafe.Sizeof(CrossType{}), 4},
		{"FourNibbles", unsafe.Sizeof(FourNibbles{}), 1},
		{"Wide32", unsafe.Sizeof(Wide32{}), 4},
		{"Wide64", unsafe.Sizeof(Wide64{}), 8},
		{"SingleBit", unsafe.Sizeof(SingleBit{}), 1},
	}
	for _, tt := range tests {
		if tt.size != tt.want {
			t.Errorf("%s: sizeof=%d, want %d", tt.name, tt.size, tt.want)
		}
	}
}

// --- Basic get/set ---

func TestSetGetUint8(t *testing.T) {
	var s TwoFields8

	// A is 3 bits → range [0, 7]
	for i := uint8(0); i <= 7; i++ {
		s.A = i
		if s.A != i {
			t.Errorf("A: set %d, got %d", i, s.A)
		}
	}

	// B is 5 bits → range [0, 31]
	for i := uint8(0); i <= 31; i++ {
		s.B = i
		if s.B != i {
			t.Errorf("B: set %d, got %d", i, s.B)
		}
	}
}

func TestSetGetUint16(t *testing.T) {
	var s CrossType

	s.A = 7
	s.B = 4095 // 2^12 - 1
	if s.A != 7 {
		t.Errorf("A: got %d, want 7", s.A)
	}
	if s.B != 4095 {
		t.Errorf("B: got %d, want 4095", s.B)
	}
}

func TestSetGetUint32(t *testing.T) {
	var s Wide32

	s.A = 0xFFFF
	s.B = 0xABCD
	if s.A != 0xFFFF {
		t.Errorf("A: got 0x%x, want 0xFFFF", s.A)
	}
	if s.B != 0xABCD {
		t.Errorf("B: got 0x%x, want 0xABCD", s.B)
	}
}

func TestSetGetUint64(t *testing.T) {
	var s Wide64

	s.A = (1 << 33) - 1 // max for 33 bits
	s.B = (1 << 31) - 1 // max for 31 bits
	if s.A != (1<<33)-1 {
		t.Errorf("A: got %d, want %d", s.A, uint64((1<<33)-1))
	}
	if s.B != (1<<31)-1 {
		t.Errorf("B: got %d, want %d", s.B, uint64((1<<31)-1))
	}
}

func TestSetGetFullWidth(t *testing.T) {
	var s FullWidth

	s.A = 255
	s.B = 65535
	s.C = 0xDEADBEEF
	if s.A != 255 {
		t.Errorf("A: got %d", s.A)
	}
	if s.B != 65535 {
		t.Errorf("B: got %d", s.B)
	}
	if s.C != 0xDEADBEEF {
		t.Errorf("C: got 0x%x", s.C)
	}
}

// --- Field isolation: setting one field must not corrupt another ---

func TestFieldIsolation(t *testing.T) {
	var s TwoFields8

	s.A = 5
	s.B = 17

	// Overwrite A, B must remain
	s.A = 3
	if s.B != 17 {
		t.Errorf("changing A corrupted B: got %d, want 17", s.B)
	}

	// Overwrite B, A must remain
	s.B = 30
	if s.A != 3 {
		t.Errorf("changing B corrupted A: got %d, want 3", s.A)
	}
}

func TestFieldIsolation4Fields(t *testing.T) {
	var s FourNibbles

	s.A = 3
	s.B = 2
	s.C = 1
	s.D = 0

	// Verify all
	if s.A != 3 || s.B != 2 || s.C != 1 || s.D != 0 {
		t.Errorf("initial: A=%d B=%d C=%d D=%d", s.A, s.B, s.C, s.D)
	}

	// Change middle fields
	s.B = 0
	s.C = 3
	if s.A != 3 {
		t.Errorf("A corrupted: got %d", s.A)
	}
	if s.D != 0 {
		t.Errorf("D corrupted: got %d", s.D)
	}
}

func TestFieldIsolationCrossUnit(t *testing.T) {
	// Overflow8: A(3 bits) in unit0, B(6 bits) in unit1
	var s Overflow8

	s.A = 7
	s.B = 63
	if s.A != 7 {
		t.Errorf("A: got %d", s.A)
	}
	if s.B != 63 {
		t.Errorf("B: got %d", s.B)
	}

	s.A = 0
	if s.B != 63 {
		t.Errorf("changing A corrupted B: got %d", s.B)
	}

	s.B = 0
	if s.A != 0 {
		t.Errorf("changing B corrupted A: got %d", s.A)
	}
}

// --- Truncation: values exceeding bit width are masked ---

func TestTruncation(t *testing.T) {
	var s TwoFields8
	val := uint8(0xFF) // use a variable to bypass compile-time overflow check

	// A is 3 bits: 0xFF → should keep low 3 bits = 7
	s.A = val
	if s.A != 7 {
		t.Errorf("A truncation: got %d, want 7", s.A)
	}

	// B is 5 bits: 0xFF → should keep low 5 bits = 31
	s.B = val
	if s.B != 31 {
		t.Errorf("B truncation: got %d, want 31", s.B)
	}
}

// --- Single bit field ---

func TestSingleBit(t *testing.T) {
	var s SingleBit

	s.Flag = 0
	if s.Flag != 0 {
		t.Errorf("Flag: got %d, want 0", s.Flag)
	}

	s.Flag = 1
	if s.Flag != 1 {
		t.Errorf("Flag: got %d, want 1", s.Flag)
	}

	// Truncation: 0xFF → 1 (use variable to bypass compile-time overflow check)
	flagVal := uint8(0xFF)
	s.Flag = flagVal
	if s.Flag != 1 {
		t.Errorf("Flag truncation: got %d, want 1", s.Flag)
	}
}

// --- Signed types ---

func TestSignedRange(t *testing.T) {
	var s SignedSmall

	// A is 3-bit signed: range [-4, 3]
	for i := int8(-4); i <= 3; i++ {
		s.A = i
		if s.A != i {
			t.Errorf("A: set %d, got %d", i, s.A)
		}
	}

	// B is 5-bit signed: range [-16, 15]
	for i := int8(-16); i <= 15; i++ {
		s.B = i
		if s.B != i {
			t.Errorf("B: set %d, got %d", i, s.B)
		}
	}
}

func TestSigned16(t *testing.T) {
	var s Signed16

	// 10-bit signed: range [-512, 511]
	s.Val = -512
	if s.Val != -512 {
		t.Errorf("min: got %d, want -512", s.Val)
	}
	s.Val = 511
	if s.Val != 511 {
		t.Errorf("max: got %d, want 511", s.Val)
	}
	s.Val = -1
	if s.Val != -1 {
		t.Errorf("-1: got %d", s.Val)
	}
}

func TestSigned32(t *testing.T) {
	var s Signed32

	// 20-bit signed: range [-524288, 524287]
	s.Val = -524288
	if s.Val != -524288 {
		t.Errorf("min: got %d", s.Val)
	}
	s.Val = 524287
	if s.Val != 524287 {
		t.Errorf("max: got %d", s.Val)
	}
}

func TestSingleBitSigned(t *testing.T) {
	var s SingleBitSigned

	// 1-bit signed: 0 → 0, 1 → -1
	s.Flag = 0
	if s.Flag != 0 {
		t.Errorf("0: got %d", s.Flag)
	}
	s.Flag = -1
	if s.Flag != -1 {
		t.Errorf("-1: got %d", s.Flag)
	}
}

// --- Compound operations ---

func TestIncrement(t *testing.T) {
	var s TwoFields8
	s.A = 3
	s.A++
	if s.A != 4 {
		t.Errorf("A after ++: got %d, want 4", s.A)
	}
}

func TestDecrement(t *testing.T) {
	var s TwoFields8
	s.A = 3
	s.A--
	if s.A != 2 {
		t.Errorf("A after --: got %d, want 2", s.A)
	}
}

func TestCompoundAdd(t *testing.T) {
	var s TwoFields8
	s.B = 10
	s.B += 5
	if s.B != 15 {
		t.Errorf("B after +=5: got %d, want 15", s.B)
	}
}

func TestCompoundSub(t *testing.T) {
	var s TwoFields8
	s.B = 20
	s.B -= 7
	if s.B != 13 {
		t.Errorf("B after -=7: got %d, want 13", s.B)
	}
}

func TestCompoundOr(t *testing.T) {
	var s TwoFields8
	s.B = 0b10101
	s.B |= 0b01010
	if s.B != 0b11111 {
		t.Errorf("B after |=: got 0b%05b, want 0b11111", s.B)
	}
}

func TestCompoundAnd(t *testing.T) {
	var s TwoFields8
	s.B = 0b11111
	s.B &= 0b10101
	if s.B != 0b10101 {
		t.Errorf("B after &=: got 0b%05b, want 0b10101", s.B)
	}
}

func TestCompoundXor(t *testing.T) {
	var s TwoFields8
	s.B = 0b11000
	s.B ^= 0b10100
	if s.B != 0b01100 {
		t.Errorf("B after ^=: got 0b%05b, want 0b01100", s.B)
	}
}

func TestCompoundShiftLeft(t *testing.T) {
	var s TwoFields8
	s.B = 3
	s.B <<= 2
	if s.B != 12 {
		t.Errorf("B after <<=2: got %d, want 12", s.B)
	}
}

func TestCompoundShiftRight(t *testing.T) {
	var s TwoFields8
	s.B = 28
	s.B >>= 2
	if s.B != 7 {
		t.Errorf("B after >>=2: got %d, want 7", s.B)
	}
}

func TestCompoundMul(t *testing.T) {
	var s TwoFields8
	s.B = 6
	s.B *= 3
	if s.B != 18 {
		t.Errorf("B after *=3: got %d, want 18", s.B)
	}
}

func TestCompoundDiv(t *testing.T) {
	var s TwoFields8
	s.B = 20
	s.B /= 4
	if s.B != 5 {
		t.Errorf("B after /=4: got %d, want 5", s.B)
	}
}

func TestCompoundRem(t *testing.T) {
	var s TwoFields8
	s.B = 17
	s.B %= 5
	if s.B != 2 {
		t.Errorf("B after %%=5: got %d, want 2", s.B)
	}
}

func TestCompoundAndNot(t *testing.T) {
	var s TwoFields8
	s.B = 0b11111
	s.B &^= 0b01010
	if s.B != 0b10101 {
		t.Errorf("B after &^=: got 0b%05b, want 0b10101", s.B)
	}
}

// --- Composite literal ---

func TestCompositeLiteral(t *testing.T) {
	s := TwoFields8{
		A: 5,
		B: 17,
	}
	if s.A != 5 {
		t.Errorf("A: got %d, want 5", s.A)
	}
	if s.B != 17 {
		t.Errorf("B: got %d, want 17", s.B)
	}
}

func TestCompositeLiteralPartial(t *testing.T) {
	// Only set one field; the other should be zero
	s := TwoFields8{
		B: 25,
	}
	if s.A != 0 {
		t.Errorf("A: got %d, want 0", s.A)
	}
	if s.B != 25 {
		t.Errorf("B: got %d, want 25", s.B)
	}
}

// --- Use in expressions ---

func TestUseInExpression(t *testing.T) {
	var s TwoFields8
	s.A = 5
	s.B = 10
	sum := s.A + s.B
	if sum != 15 {
		t.Errorf("sum: got %d, want 15", sum)
	}
}

func TestUseAsArgument(t *testing.T) {
	var s TwoFields8
	s.A = 7
	result := double(s.A)
	if result != 14 {
		t.Errorf("double: got %d, want 14", result)
	}
}

func double(v uint8) uint8 { return v * 2 }

func TestUseInCondition(t *testing.T) {
	var s SingleBit
	s.Flag = 1
	if s.Flag != 1 {
		t.Error("condition failed")
	}
}

// --- Zero value ---

func TestZeroValue(t *testing.T) {
	var s TwoFields8
	if s.A != 0 || s.B != 0 {
		t.Errorf("zero value: A=%d B=%d", s.A, s.B)
	}
}

// --- Sequential writes ---

func TestSequentialWrites(t *testing.T) {
	var s FourNibbles
	// Write all permutations of 2-bit values
	for a := uint8(0); a < 4; a++ {
		for b := uint8(0); b < 4; b++ {
			for c := uint8(0); c < 4; c++ {
				for d := uint8(0); d < 4; d++ {
					s.A = a
					s.B = b
					s.C = c
					s.D = d
					if s.A != a || s.B != b || s.C != c || s.D != d {
						t.Fatalf("A=%d B=%d C=%d D=%d → got A=%d B=%d C=%d D=%d",
							a, b, c, d, s.A, s.B, s.C, s.D)
					}
				}
			}
		}
	}
}
