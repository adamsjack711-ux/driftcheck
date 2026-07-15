// Package model defines the normalized representation every config format
// is parsed into: a flat map of key paths to typed leaf values.
package model

import (
	"fmt"
	"strconv"
	"time"
)

// Kind is the normalized type of a leaf value.
type Kind int

const (
	KindNull Kind = iota
	KindBool
	KindInt
	KindFloat
	KindString
	// KindEmptyMap / KindEmptyList are leaves for {} and []. Without them an
	// empty container would flatten to nothing and be indistinguishable from
	// the key not existing at all.
	KindEmptyMap
	KindEmptyList
)

func (k Kind) String() string {
	switch k {
	case KindNull:
		return "null"
	case KindBool:
		return "bool"
	case KindInt:
		return "int"
	case KindFloat:
		return "float"
	case KindString:
		return "string"
	case KindEmptyMap:
		return "empty map"
	case KindEmptyList:
		return "empty list"
	default:
		return "unknown"
	}
}

// Value is a normalized leaf value. Exactly one payload field is meaningful,
// selected by Kind.
type Value struct {
	Kind  Kind
	Bool  bool
	Int   int64
	Float float64
	Str   string
}

func Null() Value              { return Value{Kind: KindNull} }
func BoolVal(b bool) Value     { return Value{Kind: KindBool, Bool: b} }
func IntVal(i int64) Value     { return Value{Kind: KindInt, Int: i} }
func FloatVal(f float64) Value { return Value{Kind: KindFloat, Float: f} }
func StringVal(s string) Value { return Value{Kind: KindString, Str: s} }
func EmptyMap() Value          { return Value{Kind: KindEmptyMap} }
func EmptyList() Value         { return Value{Kind: KindEmptyList} }

// Canonical returns the value rendered as a plain string, with no type
// decoration. Two values whose Canonical forms match but whose Kinds differ
// are a type drift, not a value drift.
func (v Value) Canonical() string {
	switch v.Kind {
	case KindNull:
		return "null"
	case KindBool:
		return strconv.FormatBool(v.Bool)
	case KindInt:
		return strconv.FormatInt(v.Int, 10)
	case KindFloat:
		return strconv.FormatFloat(v.Float, 'g', -1, 64)
	case KindString:
		return v.Str
	case KindEmptyMap:
		return "{}"
	case KindEmptyList:
		return "[]"
	default:
		return ""
	}
}

// Display returns the value formatted for human output: strings quoted,
// everything else canonical.
func (v Value) Display() string {
	if v.Kind == KindString {
		return strconv.Quote(v.Str)
	}
	return v.Canonical()
}

// Equal reports whether two values have the same kind and payload.
func (v Value) Equal(o Value) bool {
	if v.Kind != o.Kind {
		return false
	}
	switch v.Kind {
	case KindNull, KindEmptyMap, KindEmptyList:
		return true
	case KindBool:
		return v.Bool == o.Bool
	case KindInt:
		return v.Int == o.Int
	case KindFloat:
		return v.Float == o.Float
	case KindString:
		return v.Str == o.Str
	default:
		return false
	}
}

// FromGo normalizes a value produced by one of the format decoders
// (encoding/json, yaml.v3, BurntSushi/toml) into a Value. Container types
// are not accepted here; Flatten handles recursion.
func FromGo(x any) (Value, bool) {
	switch t := x.(type) {
	case nil:
		return Null(), true
	case bool:
		return BoolVal(t), true
	case int:
		return IntVal(int64(t)), true
	case int64:
		return IntVal(t), true
	case uint64:
		if t <= 1<<63-1 {
			return IntVal(int64(t)), true
		}
		return StringVal(strconv.FormatUint(t, 10)), true
	case float64:
		// yaml/json decoders hand back float64 for anything non-integral;
		// keep integral floats as floats so "8080 vs 8080.0" is visible.
		return FloatVal(t), true
	case float32:
		return FloatVal(float64(t)), true
	case string:
		return StringVal(t), true
	case time.Time:
		// YAML timestamps and TOML datetimes normalize to RFC 3339 strings.
		return StringVal(t.Format(time.RFC3339)), true
	default:
		return Value{}, false
	}
}

// FromGoAlways is FromGo with a fmt.Sprint fallback for exotic decoder types.
func FromGoAlways(x any) Value {
	if v, ok := FromGo(x); ok {
		return v
	}
	return StringVal(fmt.Sprint(x))
}
