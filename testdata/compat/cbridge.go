package compat

/*
#include <stdint.h>
#include <stddef.h>
#include <string.h>

// --- C bitfield structs mirroring Go declarations ---

struct TwoFields8 {
	uint8_t a : 3;
	uint8_t b : 5;
};

struct Overflow8 {
	uint8_t a : 3;
	uint8_t b : 6;
};

struct CrossType {
	uint8_t  a : 3;
	uint16_t b : 12;
};

struct FourNibbles {
	uint8_t a : 2;
	uint8_t b : 2;
	uint8_t c : 2;
	uint8_t d : 2;
};

struct Wide32 {
	uint32_t a : 16;
	uint32_t b : 16;
};

struct Wide64 {
	uint64_t a : 33;
	uint64_t b : 31;
};

struct IPv4Header {
	uint8_t  version : 4;
	uint8_t  ihl     : 4;
	uint8_t  dscp    : 6;
	uint8_t  ecn     : 2;
	uint16_t length  : 16;
};

struct SignedSmall {
	int8_t a : 3;
	int8_t b : 5;
};

struct Signed16 {
	int16_t val : 10;
};

struct SingleBit {
	uint8_t flag : 1;
};

struct SingleBitSigned {
	int8_t flag : 1;
};

// --- Mixed structs: bitfields + regular fields ---

// Regular field in the middle
struct MixedMiddle {
	uint8_t  flags : 4;
	uint8_t  mode  : 4;
	uint32_t count;
	uint8_t  tag   : 3;
	uint8_t  valid : 1;
};

// Regular field at the start
struct MixedStart {
	uint32_t id;
	uint8_t  version : 4;
	uint8_t  ihl     : 4;
};

// Regular field at the end
struct MixedEnd {
	uint8_t  version : 4;
	uint8_t  ihl     : 4;
	uint32_t payload;
};

// Multiple regular fields separating bitfield groups
struct MixedMulti {
	uint8_t  a : 3;
	uint16_t x;
	uint8_t  b : 5;
	uint32_t y;
	uint8_t  c : 1;
};

// Regular field between same-type bitfield groups
struct MixedSameType {
	uint8_t hi  : 4;
	uint8_t sep;
	uint8_t lo  : 4;
};

// --- sizeof ---

static size_t c_sizeof_TwoFields8(void)      { return sizeof(struct TwoFields8); }
static size_t c_sizeof_Overflow8(void)        { return sizeof(struct Overflow8); }
static size_t c_sizeof_CrossType(void)        { return sizeof(struct CrossType); }
static size_t c_sizeof_FourNibbles(void)      { return sizeof(struct FourNibbles); }
static size_t c_sizeof_Wide32(void)           { return sizeof(struct Wide32); }
static size_t c_sizeof_Wide64(void)           { return sizeof(struct Wide64); }
static size_t c_sizeof_IPv4Header(void)       { return sizeof(struct IPv4Header); }
static size_t c_sizeof_SignedSmall(void)      { return sizeof(struct SignedSmall); }
static size_t c_sizeof_Signed16(void)         { return sizeof(struct Signed16); }
static size_t c_sizeof_SingleBit(void)        { return sizeof(struct SingleBit); }
static size_t c_sizeof_SingleBitSigned(void)  { return sizeof(struct SingleBitSigned); }
static size_t c_sizeof_MixedMiddle(void)      { return sizeof(struct MixedMiddle); }
static size_t c_sizeof_MixedStart(void)       { return sizeof(struct MixedStart); }
static size_t c_sizeof_MixedEnd(void)         { return sizeof(struct MixedEnd); }
static size_t c_sizeof_MixedMulti(void)       { return sizeof(struct MixedMulti); }
static size_t c_sizeof_MixedSameType(void)    { return sizeof(struct MixedSameType); }

// --- Write fields from C → raw bytes ---

static void c_write_TwoFields8(void *buf, uint8_t a, uint8_t b) {
	struct TwoFields8 s = {0};
	s.a = a; s.b = b;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_TwoFields8(const void *buf, uint8_t *a, uint8_t *b) {
	struct TwoFields8 s;
	memcpy(&s, buf, sizeof(s));
	*a = s.a; *b = s.b;
}

static void c_write_IPv4Header(void *buf, uint8_t version, uint8_t ihl,
                                uint8_t dscp, uint8_t ecn, uint16_t length) {
	struct IPv4Header s = {0};
	s.version = version; s.ihl = ihl;
	s.dscp = dscp; s.ecn = ecn; s.length = length;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_IPv4Header(const void *buf, uint8_t *version, uint8_t *ihl,
                               uint8_t *dscp, uint8_t *ecn, uint16_t *length) {
	struct IPv4Header s;
	memcpy(&s, buf, sizeof(s));
	*version = s.version; *ihl = s.ihl;
	*dscp = s.dscp; *ecn = s.ecn; *length = s.length;
}

static void c_write_Wide32(void *buf, uint32_t a, uint32_t b) {
	struct Wide32 s = {0};
	s.a = a; s.b = b;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_Wide32(const void *buf, uint32_t *a, uint32_t *b) {
	struct Wide32 s;
	memcpy(&s, buf, sizeof(s));
	*a = s.a; *b = s.b;
}

static void c_write_FourNibbles(void *buf, uint8_t a, uint8_t b, uint8_t c, uint8_t d) {
	struct FourNibbles s = {0};
	s.a = a; s.b = b; s.c = c; s.d = d;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_FourNibbles(const void *buf, uint8_t *a, uint8_t *b, uint8_t *c, uint8_t *d) {
	struct FourNibbles s;
	memcpy(&s, buf, sizeof(s));
	*a = s.a; *b = s.b; *c = s.c; *d = s.d;
}

static void c_write_SignedSmall(void *buf, int8_t a, int8_t b) {
	struct SignedSmall s = {0};
	s.a = a; s.b = b;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_SignedSmall(const void *buf, int8_t *a, int8_t *b) {
	struct SignedSmall s;
	memcpy(&s, buf, sizeof(s));
	*a = s.a; *b = s.b;
}

static void c_write_Signed16(void *buf, int16_t val) {
	struct Signed16 s = {0};
	s.val = val;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_Signed16(const void *buf, int16_t *val) {
	struct Signed16 s;
	memcpy(&s, buf, sizeof(s));
	*val = s.val;
}

static void c_write_SingleBitSigned(void *buf, int8_t flag) {
	struct SingleBitSigned s = {0};
	s.flag = flag;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_SingleBitSigned(const void *buf, int8_t *flag) {
	struct SingleBitSigned s;
	memcpy(&s, buf, sizeof(s));
	*flag = s.flag;
}
// --- Mixed: write/read ---

static void c_write_MixedMiddle(void *buf, uint8_t flags, uint8_t mode,
                                 uint32_t count, uint8_t tag, uint8_t valid) {
	struct MixedMiddle s;
	memset(&s, 0, sizeof(s));
	s.flags = flags; s.mode = mode;
	s.count = count;
	s.tag = tag; s.valid = valid;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_MixedMiddle(const void *buf, uint8_t *flags, uint8_t *mode,
                                uint32_t *count, uint8_t *tag, uint8_t *valid) {
	struct MixedMiddle s;
	memcpy(&s, buf, sizeof(s));
	*flags = s.flags; *mode = s.mode;
	*count = s.count;
	*tag = s.tag; *valid = s.valid;
}

static void c_write_MixedStart(void *buf, uint32_t id, uint8_t version, uint8_t ihl) {
	struct MixedStart s;
	memset(&s, 0, sizeof(s));
	s.id = id; s.version = version; s.ihl = ihl;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_MixedStart(const void *buf, uint32_t *id, uint8_t *version, uint8_t *ihl) {
	struct MixedStart s;
	memcpy(&s, buf, sizeof(s));
	*id = s.id; *version = s.version; *ihl = s.ihl;
}

static void c_write_MixedEnd(void *buf, uint8_t version, uint8_t ihl, uint32_t payload) {
	struct MixedEnd s;
	memset(&s, 0, sizeof(s));
	s.version = version; s.ihl = ihl; s.payload = payload;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_MixedEnd(const void *buf, uint8_t *version, uint8_t *ihl, uint32_t *payload) {
	struct MixedEnd s;
	memcpy(&s, buf, sizeof(s));
	*version = s.version; *ihl = s.ihl; *payload = s.payload;
}

static void c_write_MixedMulti(void *buf, uint8_t a, uint16_t x,
                                uint8_t b, uint32_t y, uint8_t c) {
	struct MixedMulti s;
	memset(&s, 0, sizeof(s));
	s.a = a; s.x = x; s.b = b; s.y = y; s.c = c;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_MixedMulti(const void *buf, uint8_t *a, uint16_t *x,
                               uint8_t *b, uint32_t *y, uint8_t *c) {
	struct MixedMulti s;
	memcpy(&s, buf, sizeof(s));
	*a = s.a; *x = s.x; *b = s.b; *y = s.y; *c = s.c;
}

static void c_write_MixedSameType(void *buf, uint8_t hi, uint8_t sep, uint8_t lo) {
	struct MixedSameType s;
	memset(&s, 0, sizeof(s));
	s.hi = hi; s.sep = sep; s.lo = lo;
	memcpy(buf, &s, sizeof(s));
}

static void c_read_MixedSameType(const void *buf, uint8_t *hi, uint8_t *sep, uint8_t *lo) {
	struct MixedSameType s;
	memcpy(&s, buf, sizeof(s));
	*hi = s.hi; *sep = s.sep; *lo = s.lo;
}
*/
import "C"

import "unsafe"

// C sizeof functions
func CSizeofTwoFields8() uintptr      { return uintptr(C.c_sizeof_TwoFields8()) }
func CSizeofOverflow8() uintptr       { return uintptr(C.c_sizeof_Overflow8()) }
func CSizeofCrossType() uintptr       { return uintptr(C.c_sizeof_CrossType()) }
func CSizeofFourNibbles() uintptr     { return uintptr(C.c_sizeof_FourNibbles()) }
func CSizeofWide32() uintptr          { return uintptr(C.c_sizeof_Wide32()) }
func CSizeofWide64() uintptr          { return uintptr(C.c_sizeof_Wide64()) }
func CSizeofIPv4Header() uintptr      { return uintptr(C.c_sizeof_IPv4Header()) }
func CSizeofSignedSmall() uintptr     { return uintptr(C.c_sizeof_SignedSmall()) }
func CSizeofSigned16() uintptr        { return uintptr(C.c_sizeof_Signed16()) }
func CSizeofSingleBit() uintptr       { return uintptr(C.c_sizeof_SingleBit()) }
func CSizeofSingleBitSigned() uintptr { return uintptr(C.c_sizeof_SingleBitSigned()) }

// C write/read wrappers

func CWriteTwoFields8(buf unsafe.Pointer, a, b uint8) {
	C.c_write_TwoFields8(buf, C.uint8_t(a), C.uint8_t(b))
}

func CReadTwoFields8(buf unsafe.Pointer) (uint8, uint8) {
	var a, b C.uint8_t
	C.c_read_TwoFields8(buf, &a, &b)
	return uint8(a), uint8(b)
}

func CWriteIPv4Header(buf unsafe.Pointer, version, ihl, dscp, ecn uint8, length uint16) {
	C.c_write_IPv4Header(buf, C.uint8_t(version), C.uint8_t(ihl),
		C.uint8_t(dscp), C.uint8_t(ecn), C.uint16_t(length))
}

func CReadIPv4Header(buf unsafe.Pointer) (version, ihl, dscp, ecn uint8, length uint16) {
	var v, i, d, e C.uint8_t
	var l C.uint16_t
	C.c_read_IPv4Header(buf, &v, &i, &d, &e, &l)
	return uint8(v), uint8(i), uint8(d), uint8(e), uint16(l)
}

func CWriteWide32(buf unsafe.Pointer, a, b uint32) {
	C.c_write_Wide32(buf, C.uint32_t(a), C.uint32_t(b))
}

func CReadWide32(buf unsafe.Pointer) (uint32, uint32) {
	var a, b C.uint32_t
	C.c_read_Wide32(buf, &a, &b)
	return uint32(a), uint32(b)
}

func CWriteFourNibbles(buf unsafe.Pointer, a, b, c, d uint8) {
	C.c_write_FourNibbles(buf, C.uint8_t(a), C.uint8_t(b), C.uint8_t(c), C.uint8_t(d))
}

func CReadFourNibbles(buf unsafe.Pointer) (uint8, uint8, uint8, uint8) {
	var a, b, c, d C.uint8_t
	C.c_read_FourNibbles(buf, &a, &b, &c, &d)
	return uint8(a), uint8(b), uint8(c), uint8(d)
}

func CWriteSignedSmall(buf unsafe.Pointer, a, b int8) {
	C.c_write_SignedSmall(buf, C.int8_t(a), C.int8_t(b))
}

func CReadSignedSmall(buf unsafe.Pointer) (int8, int8) {
	var a, b C.int8_t
	C.c_read_SignedSmall(buf, &a, &b)
	return int8(a), int8(b)
}

func CWriteSigned16(buf unsafe.Pointer, val int16) {
	C.c_write_Signed16(buf, C.int16_t(val))
}

func CReadSigned16(buf unsafe.Pointer) int16 {
	var v C.int16_t
	C.c_read_Signed16(buf, &v)
	return int16(v)
}

func CWriteSingleBitSigned(buf unsafe.Pointer, flag int8) {
	C.c_write_SingleBitSigned(buf, C.int8_t(flag))
}

func CReadSingleBitSigned(buf unsafe.Pointer) int8 {
	var f C.int8_t
	C.c_read_SingleBitSigned(buf, &f)
	return int8(f)
}

// Mixed struct helpers

func CSizeofMixedMiddle() uintptr   { return uintptr(C.c_sizeof_MixedMiddle()) }
func CSizeofMixedStart() uintptr    { return uintptr(C.c_sizeof_MixedStart()) }
func CSizeofMixedEnd() uintptr      { return uintptr(C.c_sizeof_MixedEnd()) }
func CSizeofMixedMulti() uintptr    { return uintptr(C.c_sizeof_MixedMulti()) }
func CSizeofMixedSameType() uintptr { return uintptr(C.c_sizeof_MixedSameType()) }

func CWriteMixedMiddle(buf unsafe.Pointer, flags, mode uint8, count uint32, tag, valid uint8) {
	C.c_write_MixedMiddle(buf, C.uint8_t(flags), C.uint8_t(mode),
		C.uint32_t(count), C.uint8_t(tag), C.uint8_t(valid))
}

func CReadMixedMiddle(buf unsafe.Pointer) (flags, mode uint8, count uint32, tag, valid uint8) {
	var f, m, tg, v C.uint8_t
	var c C.uint32_t
	C.c_read_MixedMiddle(buf, &f, &m, &c, &tg, &v)
	return uint8(f), uint8(m), uint32(c), uint8(tg), uint8(v)
}

func CWriteMixedStart(buf unsafe.Pointer, id uint32, version, ihl uint8) {
	C.c_write_MixedStart(buf, C.uint32_t(id), C.uint8_t(version), C.uint8_t(ihl))
}

func CReadMixedStart(buf unsafe.Pointer) (uint32, uint8, uint8) {
	var id C.uint32_t
	var v, i C.uint8_t
	C.c_read_MixedStart(buf, &id, &v, &i)
	return uint32(id), uint8(v), uint8(i)
}

func CWriteMixedEnd(buf unsafe.Pointer, version, ihl uint8, payload uint32) {
	C.c_write_MixedEnd(buf, C.uint8_t(version), C.uint8_t(ihl), C.uint32_t(payload))
}

func CReadMixedEnd(buf unsafe.Pointer) (uint8, uint8, uint32) {
	var v, i C.uint8_t
	var p C.uint32_t
	C.c_read_MixedEnd(buf, &v, &i, &p)
	return uint8(v), uint8(i), uint32(p)
}

func CWriteMixedMulti(buf unsafe.Pointer, a uint8, x uint16, b uint8, y uint32, cc uint8) {
	C.c_write_MixedMulti(buf, C.uint8_t(a), C.uint16_t(x),
		C.uint8_t(b), C.uint32_t(y), C.uint8_t(cc))
}

func CReadMixedMulti(buf unsafe.Pointer) (uint8, uint16, uint8, uint32, uint8) {
	var a, b, cc C.uint8_t
	var x C.uint16_t
	var y C.uint32_t
	C.c_read_MixedMulti(buf, &a, &x, &b, &y, &cc)
	return uint8(a), uint16(x), uint8(b), uint32(y), uint8(cc)
}

func CWriteMixedSameType(buf unsafe.Pointer, hi, sep, lo uint8) {
	C.c_write_MixedSameType(buf, C.uint8_t(hi), C.uint8_t(sep), C.uint8_t(lo))
}

func CReadMixedSameType(buf unsafe.Pointer) (uint8, uint8, uint8) {
	var h, s, l C.uint8_t
	C.c_read_MixedSameType(buf, &h, &s, &l)
	return uint8(h), uint8(s), uint8(l)
}
