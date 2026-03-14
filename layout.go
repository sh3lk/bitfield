package main

import "fmt"

// BackingType represents the integer type used for storage.
type BackingType int

const (
	Uint8 BackingType = iota
	Uint16
	Uint32
	Uint64
	Int8
	Int16
	Int32
	Int64
)

func (t BackingType) String() string {
	switch t {
	case Uint8:
		return "uint8"
	case Uint16:
		return "uint16"
	case Uint32:
		return "uint32"
	case Uint64:
		return "uint64"
	case Int8:
		return "int8"
	case Int16:
		return "int16"
	case Int32:
		return "int32"
	case Int64:
		return "int64"
	default:
		return "unknown"
	}
}

// TypeSize returns the size in bytes.
func TypeSize(t BackingType) int {
	switch t {
	case Uint8, Int8:
		return 1
	case Uint16, Int16:
		return 2
	case Uint32, Int32:
		return 4
	case Uint64, Int64:
		return 8
	default:
		panic("unknown backing type")
	}
}

// TypeAlign returns the alignment in bytes (equals size for standard C ABI).
func TypeAlign(t BackingType) int {
	return TypeSize(t)
}

// IsSigned returns true for signed integer types.
func IsSigned(t BackingType) bool {
	return t >= Int8
}

// UnsignedOf returns the unsigned counterpart. Already-unsigned types are returned as-is.
func UnsignedOf(t BackingType) BackingType {
	switch t {
	case Int8:
		return Uint8
	case Int16:
		return Uint16
	case Int32:
		return Uint32
	case Int64:
		return Uint64
	default:
		return t
	}
}

// FieldDescriptor describes a single field in the source struct.
type FieldDescriptor struct {
	Name       string
	Type       BackingType
	Width      int // bit width (from `bits:"N"` tag); 0 for regular fields
	IsBitField bool
	// For regular (non-bitfield) fields only:
	Size  int // byte size
	Align int // byte alignment
}

// PlacedField is a bitfield placed within a StorageUnit.
type PlacedField struct {
	Name      string
	Type      BackingType // original type from source (may be signed)
	Width     int         // bit width
	BitOffset int         // bit offset within the storage unit
	Signed    bool
}

// StorageUnit is a contiguous block of storage backing one or more bitfields.
type StorageUnit struct {
	Index      int
	Type       BackingType // always unsigned
	ByteOffset int
	Fields     []PlacedField
	UsedBits   int
}

// Layout is the computed C-ABI-compatible memory layout of a struct.
type Layout struct {
	Units     []StorageUnit
	TotalSize int
	MaxAlign  int
}

// ValidateField checks that a FieldDescriptor is valid.
// Returns an error with context for user-facing diagnostics.
func ValidateField(f FieldDescriptor) error {
	if !f.IsBitField {
		return nil
	}
	if f.Width <= 0 {
		return fmt.Errorf("field %q: bit width must be > 0, got %d", f.Name, f.Width)
	}
	maxBits := TypeSize(f.Type) * 8
	if f.Width > maxBits {
		return fmt.Errorf("field %q: bit width %d exceeds %s size (%d bits)",
			f.Name, f.Width, f.Type, maxBits)
	}
	return nil
}

// ComputeLayout computes the C-ABI-compatible layout for a sequence of fields.
//
// Algorithm (from PRD):
//
//	For each field:
//	  - Regular field: advance offset by sizeof(field), update maxAlign, reset current unit.
//	  - Bitfield: if backing type changed, or bits don't fit, start a new storage unit
//	    aligned to alignof(backing_type). Place the field at the current bit offset.
//	  - Trailing padding: round totalSize up to maxAlign.
func ComputeLayout(fields []FieldDescriptor) (Layout, error) {
	for _, f := range fields {
		if err := ValidateField(f); err != nil {
			return Layout{}, err
		}
	}

	var (
		currentOffset  int
		maxAlign       = 1
		currentUnitIdx = -1
		units          []StorageUnit
	)

	for _, f := range fields {
		if !f.IsBitField {
			// Regular field: track offset and alignment impact.
			// We don't align the regular field here — Go handles that.
			// We do update maxAlign so trailing padding is correct.
			currentOffset += f.Size
			if f.Align > maxAlign {
				maxAlign = f.Align
			}
			currentUnitIdx = -1
			continue
		}

		backingType := UnsignedOf(f.Type)
		align := TypeAlign(backingType)
		sizeBits := TypeSize(backingType) * 8

		needNewUnit := currentUnitIdx < 0 ||
			units[currentUnitIdx].Type != backingType ||
			units[currentUnitIdx].UsedBits+f.Width > sizeBits

		if needNewUnit {
			// Leading padding / alignment.
			currentOffset = alignUp(currentOffset, align)
			if align > maxAlign {
				maxAlign = align
			}

			idx := len(units)
			units = append(units, StorageUnit{
				Index:      idx,
				Type:       backingType,
				ByteOffset: currentOffset,
			})
			currentUnitIdx = idx
			currentOffset += TypeSize(backingType)
		}

		units[currentUnitIdx].Fields = append(units[currentUnitIdx].Fields, PlacedField{
			Name:      f.Name,
			Type:      f.Type,
			Width:     f.Width,
			BitOffset: units[currentUnitIdx].UsedBits,
			Signed:    IsSigned(f.Type),
		})
		units[currentUnitIdx].UsedBits += f.Width
	}

	// Trailing padding.
	totalSize := alignUp(currentOffset, maxAlign)

	return Layout{
		Units:     units,
		TotalSize: totalSize,
		MaxAlign:  maxAlign,
	}, nil
}

func alignUp(offset, align int) int {
	return (offset + align - 1) &^ (align - 1)
}
