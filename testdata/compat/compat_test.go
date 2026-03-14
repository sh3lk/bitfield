package compat

import (
	"testing"
	"unsafe"
)

// --- Go bitfield structs (must match C structs in cbridge.go) ---

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

type IPv4Header struct {
	Version uint8  `bits:"4"`
	IHL     uint8  `bits:"4"`
	DSCP    uint8  `bits:"6"`
	ECN     uint8  `bits:"2"`
	Length  uint16 `bits:"16"`
}

type SignedSmall struct {
	A int8 `bits:"3"`
	B int8 `bits:"5"`
}

type Signed16 struct {
	Val int16 `bits:"10"`
}

type SingleBit struct {
	Flag uint8 `bits:"1"`
}

type SingleBitSigned struct {
	Flag int8 `bits:"1"`
}

// --- Mixed structs ---

type MixedMiddle struct {
	Flags uint8 `bits:"4"`
	Mode  uint8 `bits:"4"`
	Count uint32
	Tag   uint8 `bits:"3"`
	Valid uint8 `bits:"1"`
}

type MixedStart struct {
	ID      uint32
	Version uint8 `bits:"4"`
	IHL     uint8 `bits:"4"`
}

type MixedEnd struct {
	Version uint8 `bits:"4"`
	IHL     uint8 `bits:"4"`
	Payload uint32
}

type MixedMulti struct {
	A uint8 `bits:"3"`
	X uint16
	B uint8 `bits:"5"`
	Y uint32
	C uint8 `bits:"1"`
}

type MixedSameType struct {
	Hi  uint8 `bits:"4"`
	Sep uint8
	Lo  uint8 `bits:"4"`
}

// ===================== sizeof: Go must match C =====================

func TestSizeofMatch(t *testing.T) {
	tests := []struct {
		name   string
		goSize uintptr
		cSize  uintptr
	}{
		{"TwoFields8", unsafe.Sizeof(TwoFields8{}), CSizeofTwoFields8()},
		{"Overflow8", unsafe.Sizeof(Overflow8{}), CSizeofOverflow8()},
		{"CrossType", unsafe.Sizeof(CrossType{}), CSizeofCrossType()},
		{"FourNibbles", unsafe.Sizeof(FourNibbles{}), CSizeofFourNibbles()},
		{"Wide32", unsafe.Sizeof(Wide32{}), CSizeofWide32()},
		{"Wide64", unsafe.Sizeof(Wide64{}), CSizeofWide64()},
		{"IPv4Header", unsafe.Sizeof(IPv4Header{}), CSizeofIPv4Header()},
		{"SignedSmall", unsafe.Sizeof(SignedSmall{}), CSizeofSignedSmall()},
		{"Signed16", unsafe.Sizeof(Signed16{}), CSizeofSigned16()},
		{"SingleBit", unsafe.Sizeof(SingleBit{}), CSizeofSingleBit()},
		{"SingleBitSigned", unsafe.Sizeof(SingleBitSigned{}), CSizeofSingleBitSigned()},
		// Mixed structs
		{"MixedMiddle", unsafe.Sizeof(MixedMiddle{}), CSizeofMixedMiddle()},
		{"MixedStart", unsafe.Sizeof(MixedStart{}), CSizeofMixedStart()},
		{"MixedEnd", unsafe.Sizeof(MixedEnd{}), CSizeofMixedEnd()},
		{"MixedMulti", unsafe.Sizeof(MixedMulti{}), CSizeofMixedMulti()},
		{"MixedSameType", unsafe.Sizeof(MixedSameType{}), CSizeofMixedSameType()},
	}
	for _, tt := range tests {
		if tt.goSize != tt.cSize {
			t.Errorf("%s: Go sizeof=%d, C sizeof=%d", tt.name, tt.goSize, tt.cSize)
		}
	}
}

// ===================== Go writes → C reads =====================

func TestGoWriteCRead_TwoFields8(t *testing.T) {
	var s TwoFields8
	s.A = 5
	s.B = 17

	a, b := CReadTwoFields8(unsafe.Pointer(&s))
	if a != 5 {
		t.Errorf("C read A: got %d, want 5", a)
	}
	if b != 17 {
		t.Errorf("C read B: got %d, want 17", b)
	}
}

func TestGoWriteCRead_IPv4Header(t *testing.T) {
	var h IPv4Header
	h.Version = 4
	h.IHL = 5
	h.DSCP = 46
	h.ECN = 3
	h.Length = 1500

	version, ihl, dscp, ecn, length := CReadIPv4Header(unsafe.Pointer(&h))
	if version != 4 {
		t.Errorf("C read Version: got %d", version)
	}
	if ihl != 5 {
		t.Errorf("C read IHL: got %d", ihl)
	}
	if dscp != 46 {
		t.Errorf("C read DSCP: got %d", dscp)
	}
	if ecn != 3 {
		t.Errorf("C read ECN: got %d", ecn)
	}
	if length != 1500 {
		t.Errorf("C read Length: got %d", length)
	}
}

func TestGoWriteCRead_Wide32(t *testing.T) {
	var s Wide32
	s.A = 0xABCD
	s.B = 0x1234

	a, b := CReadWide32(unsafe.Pointer(&s))
	if a != 0xABCD {
		t.Errorf("C read A: got 0x%x, want 0xABCD", a)
	}
	if b != 0x1234 {
		t.Errorf("C read B: got 0x%x, want 0x1234", b)
	}
}

func TestGoWriteCRead_FourNibbles(t *testing.T) {
	var s FourNibbles
	s.A = 3
	s.B = 2
	s.C = 1
	s.D = 0

	a, b, c, d := CReadFourNibbles(unsafe.Pointer(&s))
	if a != 3 || b != 2 || c != 1 || d != 0 {
		t.Errorf("C read: A=%d B=%d C=%d D=%d, want 3 2 1 0", a, b, c, d)
	}
}

func TestGoWriteCRead_SignedSmall(t *testing.T) {
	var s SignedSmall
	s.A = -3
	s.B = -10

	a, b := CReadSignedSmall(unsafe.Pointer(&s))
	if a != -3 {
		t.Errorf("C read A: got %d, want -3", a)
	}
	if b != -10 {
		t.Errorf("C read B: got %d, want -10", b)
	}
}

func TestGoWriteCRead_Signed16(t *testing.T) {
	var s Signed16
	s.Val = -100

	val := CReadSigned16(unsafe.Pointer(&s))
	if val != -100 {
		t.Errorf("C read Val: got %d, want -100", val)
	}
}

func TestGoWriteCRead_SingleBitSigned(t *testing.T) {
	var s SingleBitSigned
	s.Flag = -1

	flag := CReadSingleBitSigned(unsafe.Pointer(&s))
	if flag != -1 {
		t.Errorf("C read Flag: got %d, want -1", flag)
	}
}

// ===================== C writes → Go reads =====================

func TestCWriteGoRead_TwoFields8(t *testing.T) {
	var s TwoFields8
	CWriteTwoFields8(unsafe.Pointer(&s), 7, 31)

	if s.A != 7 {
		t.Errorf("Go read A: got %d, want 7", s.A)
	}
	if s.B != 31 {
		t.Errorf("Go read B: got %d, want 31", s.B)
	}
}

func TestCWriteGoRead_IPv4Header(t *testing.T) {
	var h IPv4Header
	CWriteIPv4Header(unsafe.Pointer(&h), 4, 5, 46, 3, 1500)

	if h.Version != 4 {
		t.Errorf("Go read Version: got %d", h.Version)
	}
	if h.IHL != 5 {
		t.Errorf("Go read IHL: got %d", h.IHL)
	}
	if h.DSCP != 46 {
		t.Errorf("Go read DSCP: got %d", h.DSCP)
	}
	if h.ECN != 3 {
		t.Errorf("Go read ECN: got %d", h.ECN)
	}
	if h.Length != 1500 {
		t.Errorf("Go read Length: got %d", h.Length)
	}
}

func TestCWriteGoRead_Wide32(t *testing.T) {
	var s Wide32
	CWriteWide32(unsafe.Pointer(&s), 0xFFFF, 0xBEEF)

	if s.A != 0xFFFF {
		t.Errorf("Go read A: got 0x%x", s.A)
	}
	if s.B != 0xBEEF {
		t.Errorf("Go read B: got 0x%x", s.B)
	}
}

func TestCWriteGoRead_FourNibbles(t *testing.T) {
	var s FourNibbles
	CWriteFourNibbles(unsafe.Pointer(&s), 1, 2, 3, 0)

	if s.A != 1 || s.B != 2 || s.C != 3 || s.D != 0 {
		t.Errorf("Go read: A=%d B=%d C=%d D=%d, want 1 2 3 0", s.A, s.B, s.C, s.D)
	}
}

func TestCWriteGoRead_SignedSmall(t *testing.T) {
	var s SignedSmall
	CWriteSignedSmall(unsafe.Pointer(&s), -4, 15)

	if s.A != -4 {
		t.Errorf("Go read A: got %d, want -4", s.A)
	}
	if s.B != 15 {
		t.Errorf("Go read B: got %d, want 15", s.B)
	}
}

func TestCWriteGoRead_Signed16(t *testing.T) {
	var s Signed16
	CWriteSigned16(unsafe.Pointer(&s), -256)

	if s.Val != -256 {
		t.Errorf("Go read Val: got %d, want -256", s.Val)
	}
}

func TestCWriteGoRead_SingleBitSigned(t *testing.T) {
	var s SingleBitSigned
	CWriteSingleBitSigned(unsafe.Pointer(&s), -1)

	if s.Flag != -1 {
		t.Errorf("Go read Flag: got %d, want -1", s.Flag)
	}
}

// ===================== mixed: Go writes → C reads =====================

func TestGoWriteCRead_MixedMiddle(t *testing.T) {
	var s MixedMiddle
	s.Flags = 0xA
	s.Mode = 0x5
	s.Count = 0xDEADBEEF
	s.Tag = 7
	s.Valid = 1

	flags, mode, count, tag, valid := CReadMixedMiddle(unsafe.Pointer(&s))
	if flags != 0xA {
		t.Errorf("C read Flags: got %d", flags)
	}
	if mode != 0x5 {
		t.Errorf("C read Mode: got %d", mode)
	}
	if count != 0xDEADBEEF {
		t.Errorf("C read Count: got 0x%x", count)
	}
	if tag != 7 {
		t.Errorf("C read Tag: got %d", tag)
	}
	if valid != 1 {
		t.Errorf("C read Valid: got %d", valid)
	}
}

func TestGoWriteCRead_MixedStart(t *testing.T) {
	var s MixedStart
	s.ID = 0x12345678
	s.Version = 4
	s.IHL = 5

	id, version, ihl := CReadMixedStart(unsafe.Pointer(&s))
	if id != 0x12345678 {
		t.Errorf("C read ID: got 0x%x", id)
	}
	if version != 4 {
		t.Errorf("C read Version: got %d", version)
	}
	if ihl != 5 {
		t.Errorf("C read IHL: got %d", ihl)
	}
}

func TestGoWriteCRead_MixedEnd(t *testing.T) {
	var s MixedEnd
	s.Version = 6
	s.IHL = 15
	s.Payload = 0xCAFEBABE

	version, ihl, payload := CReadMixedEnd(unsafe.Pointer(&s))
	if version != 6 {
		t.Errorf("C read Version: got %d", version)
	}
	if ihl != 15 {
		t.Errorf("C read IHL: got %d", ihl)
	}
	if payload != 0xCAFEBABE {
		t.Errorf("C read Payload: got 0x%x", payload)
	}
}

func TestGoWriteCRead_MixedMulti(t *testing.T) {
	var s MixedMulti
	s.A = 5
	s.X = 1000
	s.B = 31
	s.Y = 0xBEEF
	s.C = 1

	a, x, b, y, c := CReadMixedMulti(unsafe.Pointer(&s))
	if a != 5 {
		t.Errorf("C read A: got %d", a)
	}
	if x != 1000 {
		t.Errorf("C read X: got %d", x)
	}
	if b != 31 {
		t.Errorf("C read B: got %d", b)
	}
	if y != 0xBEEF {
		t.Errorf("C read Y: got 0x%x", y)
	}
	if c != 1 {
		t.Errorf("C read C: got %d", c)
	}
}

func TestGoWriteCRead_MixedSameType(t *testing.T) {
	var s MixedSameType
	s.Hi = 0xA
	s.Sep = 0xFF
	s.Lo = 0x5

	hi, sep, lo := CReadMixedSameType(unsafe.Pointer(&s))
	if hi != 0xA {
		t.Errorf("C read Hi: got %d", hi)
	}
	if sep != 0xFF {
		t.Errorf("C read Sep: got 0x%x", sep)
	}
	if lo != 0x5 {
		t.Errorf("C read Lo: got %d", lo)
	}
}

// ===================== mixed: C writes → Go reads =====================

func TestCWriteGoRead_MixedMiddle(t *testing.T) {
	var s MixedMiddle
	CWriteMixedMiddle(unsafe.Pointer(&s), 0xF, 0x3, 42, 5, 1)

	if s.Flags != 0xF {
		t.Errorf("Go read Flags: got %d", s.Flags)
	}
	if s.Mode != 0x3 {
		t.Errorf("Go read Mode: got %d", s.Mode)
	}
	if s.Count != 42 {
		t.Errorf("Go read Count: got %d", s.Count)
	}
	if s.Tag != 5 {
		t.Errorf("Go read Tag: got %d", s.Tag)
	}
	if s.Valid != 1 {
		t.Errorf("Go read Valid: got %d", s.Valid)
	}
}

func TestCWriteGoRead_MixedStart(t *testing.T) {
	var s MixedStart
	CWriteMixedStart(unsafe.Pointer(&s), 0xAAAAAAAA, 6, 10)

	if s.ID != 0xAAAAAAAA {
		t.Errorf("Go read ID: got 0x%x", s.ID)
	}
	if s.Version != 6 {
		t.Errorf("Go read Version: got %d", s.Version)
	}
	if s.IHL != 10 {
		t.Errorf("Go read IHL: got %d", s.IHL)
	}
}

func TestCWriteGoRead_MixedEnd(t *testing.T) {
	var s MixedEnd
	CWriteMixedEnd(unsafe.Pointer(&s), 4, 5, 0xDEAD)

	if s.Version != 4 {
		t.Errorf("Go read Version: got %d", s.Version)
	}
	if s.IHL != 5 {
		t.Errorf("Go read IHL: got %d", s.IHL)
	}
	if s.Payload != 0xDEAD {
		t.Errorf("Go read Payload: got 0x%x", s.Payload)
	}
}

func TestCWriteGoRead_MixedMulti(t *testing.T) {
	var s MixedMulti
	CWriteMixedMulti(unsafe.Pointer(&s), 7, 500, 20, 0xFACE, 1)

	if s.A != 7 {
		t.Errorf("Go read A: got %d", s.A)
	}
	if s.X != 500 {
		t.Errorf("Go read X: got %d", s.X)
	}
	if s.B != 20 {
		t.Errorf("Go read B: got %d", s.B)
	}
	if s.Y != 0xFACE {
		t.Errorf("Go read Y: got 0x%x", s.Y)
	}
	if s.C != 1 {
		t.Errorf("Go read C: got %d", s.C)
	}
}

func TestCWriteGoRead_MixedSameType(t *testing.T) {
	var s MixedSameType
	CWriteMixedSameType(unsafe.Pointer(&s), 0xB, 0x42, 0xD)

	if s.Hi != 0xB {
		t.Errorf("Go read Hi: got %d", s.Hi)
	}
	if s.Sep != 0x42 {
		t.Errorf("Go read Sep: got 0x%x", s.Sep)
	}
	if s.Lo != 0xD {
		t.Errorf("Go read Lo: got %d", s.Lo)
	}
}

// ===================== exhaustive round-trip =====================

func TestExhaustiveRoundTrip_TwoFields8(t *testing.T) {
	for a := uint8(0); a <= 7; a++ {
		for b := uint8(0); b <= 31; b++ {
			// Go → C
			var s TwoFields8
			s.A = a
			s.B = b
			ca, cb := CReadTwoFields8(unsafe.Pointer(&s))
			if ca != a || cb != b {
				t.Fatalf("Go→C: set A=%d B=%d, C read A=%d B=%d", a, b, ca, cb)
			}

			// C → Go
			var s2 TwoFields8
			CWriteTwoFields8(unsafe.Pointer(&s2), a, b)
			if s2.A != a || s2.B != b {
				t.Fatalf("C→Go: set A=%d B=%d, Go read A=%d B=%d", a, b, s2.A, s2.B)
			}
		}
	}
}

func TestExhaustiveRoundTrip_FourNibbles(t *testing.T) {
	for a := uint8(0); a < 4; a++ {
		for b := uint8(0); b < 4; b++ {
			for c := uint8(0); c < 4; c++ {
				for d := uint8(0); d < 4; d++ {
					var s FourNibbles
					s.A = a
					s.B = b
					s.C = c
					s.D = d
					ca, cb, cc, cd := CReadFourNibbles(unsafe.Pointer(&s))
					if ca != a || cb != b || cc != c || cd != d {
						t.Fatalf("Go→C: %d %d %d %d → %d %d %d %d",
							a, b, c, d, ca, cb, cc, cd)
					}
				}
			}
		}
	}
}

func TestExhaustiveRoundTrip_SignedSmall(t *testing.T) {
	for a := int8(-4); a <= 3; a++ {
		for b := int8(-16); b <= 15; b++ {
			var s SignedSmall
			s.A = a
			s.B = b
			ca, cb := CReadSignedSmall(unsafe.Pointer(&s))
			if ca != a || cb != b {
				t.Fatalf("Go→C: A=%d B=%d, C read A=%d B=%d", a, b, ca, cb)
			}

			var s2 SignedSmall
			CWriteSignedSmall(unsafe.Pointer(&s2), a, b)
			if s2.A != a || s2.B != b {
				t.Fatalf("C→Go: A=%d B=%d, Go read A=%d B=%d", a, b, s2.A, s2.B)
			}
		}
	}
}

func TestExhaustiveRoundTrip_Signed16(t *testing.T) {
	// int16:10 → range [-512, 511]
	for val := int16(-512); val <= 511; val++ {
		// Go → C
		var s Signed16
		s.Val = val
		cv := CReadSigned16(unsafe.Pointer(&s))
		if cv != val {
			t.Fatalf("Go→C: set Val=%d, C read %d", val, cv)
		}

		// C → Go
		var s2 Signed16
		CWriteSigned16(unsafe.Pointer(&s2), val)
		if s2.Val != val {
			t.Fatalf("C→Go: set Val=%d, Go read %d", val, s2.Val)
		}
	}
}

func TestExhaustiveRoundTrip_SingleBitSigned(t *testing.T) {
	// int8:1 → range [-1, 0]
	for _, val := range []int8{-1, 0} {
		// Go → C
		var s SingleBitSigned
		s.Flag = val
		cf := CReadSingleBitSigned(unsafe.Pointer(&s))
		if cf != val {
			t.Fatalf("Go→C: set Flag=%d, C read %d", val, cf)
		}

		// C → Go
		var s2 SingleBitSigned
		CWriteSingleBitSigned(unsafe.Pointer(&s2), val)
		if s2.Flag != val {
			t.Fatalf("C→Go: set Flag=%d, Go read %d", val, s2.Flag)
		}
	}
}

// ===================== exhaustive mixed round-trip =====================

func TestExhaustiveRoundTrip_MixedMiddle(t *testing.T) {
	// Flags:4 [0,15], Mode:4 [0,15], Count: fixed, Tag:3 [0,7], Valid:1 [0,1]
	// 16*16*8*2 = 4096 combinations (with a fixed Count value)
	count := uint32(0xDEADBEEF)
	for flags := uint8(0); flags < 16; flags++ {
		for mode := uint8(0); mode < 16; mode++ {
			for tag := uint8(0); tag < 8; tag++ {
				for valid := uint8(0); valid < 2; valid++ {
					// Go → C
					var s MixedMiddle
					s.Flags = flags
					s.Mode = mode
					s.Count = count
					s.Tag = tag
					s.Valid = valid

					cf, cm, cc, ct, cv := CReadMixedMiddle(unsafe.Pointer(&s))
					if cf != flags || cm != mode || cc != count || ct != tag || cv != valid {
						t.Fatalf("Go→C: flags=%d mode=%d count=0x%x tag=%d valid=%d → "+
							"C read %d %d 0x%x %d %d",
							flags, mode, count, tag, valid, cf, cm, cc, ct, cv)
					}

					// C → Go
					var s2 MixedMiddle
					CWriteMixedMiddle(unsafe.Pointer(&s2), flags, mode, count, tag, valid)
					if s2.Flags != flags || s2.Mode != mode || s2.Count != count ||
						s2.Tag != tag || s2.Valid != valid {
						t.Fatalf("C→Go: flags=%d mode=%d count=0x%x tag=%d valid=%d → "+
							"Go read %d %d 0x%x %d %d",
							flags, mode, count, tag, valid,
							s2.Flags, s2.Mode, s2.Count, s2.Tag, s2.Valid)
					}
				}
			}
		}
	}
}

func TestExhaustiveRoundTrip_MixedSameType(t *testing.T) {
	// Hi:4 [0,15], Sep: [0,255], Lo:4 [0,15]
	// 16*256*16 = 65536 combinations
	for hi := uint8(0); hi < 16; hi++ {
		for lo := uint8(0); lo < 16; lo++ {
			for _, sep := range []uint8{0x00, 0x55, 0xAA, 0xFF} {
				var s MixedSameType
				s.Hi = hi
				s.Sep = sep
				s.Lo = lo

				ch, cs, cl := CReadMixedSameType(unsafe.Pointer(&s))
				if ch != hi || cs != sep || cl != lo {
					t.Fatalf("Go→C: hi=%d sep=0x%x lo=%d → C read %d 0x%x %d",
						hi, sep, lo, ch, cs, cl)
				}

				var s2 MixedSameType
				CWriteMixedSameType(unsafe.Pointer(&s2), hi, sep, lo)
				if s2.Hi != hi || s2.Sep != sep || s2.Lo != lo {
					t.Fatalf("C→Go: hi=%d sep=0x%x lo=%d → Go read %d 0x%x %d",
						hi, sep, lo, s2.Hi, s2.Sep, s2.Lo)
				}
			}
		}
	}
}
