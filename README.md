# bitfield

C-style bitfields for Go via `-toolexec`.

Bitfield transforms struct fields tagged with `bits:"N"` into packed bit operations at compile time. No code generation step, no runtime overhead — the compiler sees only inline shifts and masks.

## Install

```bash
go install bitfield@latest
```

## Usage

Add `-toolexec` to your build command:

```bash
go build -toolexec=bitfield ./...
go test  -toolexec=bitfield ./...
go run   -toolexec=bitfield .
```

## Defining bitfield structs

Tag integer fields with `bits:"N"` where N is the bit width:

```go
type Flags struct {
    A uint8 `bits:"3"` // 3 bits, range 0..7
    B uint8 `bits:"5"` // 5 bits, range 0..31
}

type IPv4Header struct {
    Version uint8  `bits:"4"`
    IHL     uint8  `bits:"4"`
    DSCP    uint8  `bits:"6"`
    ECN     uint8  `bits:"2"`
    Length  uint16 `bits:"16"`
}
```

Fields are packed into storage units following C ABI layout rules. The struct above occupies exactly 4 bytes, same as in C.

## Supported types

`uint8`, `uint16`, `uint32`, `uint64`, `int8`, `int16`, `int32`, `int64`

## Supported operations

```go
var f Flags

f.A = 5           // assignment
x := f.A          // read
f.A++             // increment
f.A--             // decrement
f.A += 2          // compound assignment (+=, -=, *=, /=, %=, &=, |=, ^=, &^=, <<=, >>=)

h := IPv4Header{  // composite literal
    Version: 4,
    IHL:     5,
}

&f.A              // compile error: cannot take address of bitfield
```

## Restrictions

Taking the address of a bitfield is a compile-time error — bitfields don't have their own memory address:

```go
var f Flags
p := &f.A // compile error: cannot take address of bitfield A
```

## Compile-time overflow checking

Constants that exceed a field's bit width are caught at compile time:

```go
var f Flags
f.A = 255 // compile error: constant 255 overflows bitfield A (unsigned 3-bit, range 0..7)
```

## Mixed structs

Bitfield and regular fields can coexist in the same struct:

```go
type Mixed struct {
    X     uint8  `bits:"3"`
    Y     uint8  `bits:"5"`
    Name  string           // regular field, not packed
    Z     uint16 `bits:"12"`
}
```

## How it works

Bitfield operates as a `-toolexec` wrapper around the Go compiler. On each compilation:

1. **Pass 1** — scans source files for structs with `bits:"N"` tags, computes C-ABI-compatible layout, and rewrites struct declarations replacing bitfields with packed storage units (`_bf0`, `_bf1`, ...).

2. **Pass 2** — rewrites all field accesses (`h.Version`, `h.Version = 4`, `h.Version++`, etc.) into inline bit shift/mask operations on the storage units.

3. The transformed source is written to a temp directory and passed to the real compiler.

Build cache integration works correctly — bitfield mixes its own content hash into the compiler's `-V=full` output, so the cache invalidates when bitfield is rebuilt.
