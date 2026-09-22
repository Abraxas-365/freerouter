package identity

import (
	"database/sql/driver"
	"fmt"
	"reflect"

	"github.com/Abraxas-365/freerouter/internal/errx"
	"github.com/google/uuid"
)

// ID is a validated UUID identity. The zero value is invalid.
// The phantom type parameter provides compile-time discrimination
// between entity kinds without runtime cost.
type ID[T any] struct{ v uuid.UUID }

// ParseID parses a raw string into a typed ID, returning a validation
// error if the string is not a valid UUID.
func ParseID[T any](raw string) (ID[T], error) {
	parsed, err := uuid.Parse(raw)
	if err != nil {
		return ID[T]{}, errx.Validation(tagName[T]() + " must be a valid UUID")
	}
	return ID[T]{v: parsed}, nil
}

// MustParseID is like ParseID but panics on invalid input.
// Use only in tests and static initialization.
func MustParseID[T any](raw string) ID[T] {
	id, err := ParseID[T](raw)
	if err != nil {
		panic(err)
	}
	return id
}

// NewID generates a new random UUID for the given entity kind.
func NewID[T any]() ID[T] { return ID[T]{v: uuid.New()} }

func (id ID[T]) String() string  { return id.v.String() }
func (id ID[T]) IsZero() bool    { return id.v == uuid.Nil }
func (id ID[T]) UUID() uuid.UUID { return id.v }

// MarshalText implements encoding.TextMarshaler (used by encoding/json).
func (id ID[T]) MarshalText() ([]byte, error) {
	if id.IsZero() {
		return []byte(""), nil
	}
	return []byte(id.v.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler (used by encoding/json).
func (id *ID[T]) UnmarshalText(b []byte) error {
	if len(b) == 0 {
		id.v = uuid.Nil
		return nil
	}
	parsed, err := uuid.Parse(string(b))
	if err != nil {
		return errx.Validation(tagName[T]() + " must be a valid UUID")
	}
	id.v = parsed
	return nil
}

// Value implements database/sql/driver.Valuer.
func (id ID[T]) Value() (driver.Value, error) {
	if id.IsZero() {
		return nil, nil
	}
	return id.v.String(), nil
}

// Scan implements database/sql.Scanner.
func (id *ID[T]) Scan(src any) error {
	if src == nil {
		id.v = uuid.Nil
		return nil
	}
	switch v := src.(type) {
	case string:
		parsed, err := uuid.Parse(v)
		if err != nil {
			return errx.Validation(tagName[T]() + " must be a valid UUID")
		}
		id.v = parsed
	case []byte:
		parsed, err := uuid.ParseBytes(v)
		if err != nil {
			return errx.Validation(tagName[T]() + " must be a valid UUID")
		}
		id.v = parsed
	default:
		return errx.Internal("cannot scan into " + tagName[T]())
	}
	return nil
}

// tagName returns a human-readable name for validation error messages.
func tagName[T any]() string {
	t := reflect.TypeOf((*T)(nil)).Elem()
	name := t.Name()
	if name == "" {
		return "id"
	}
	// Strip "Tag" suffix for cleaner messages
	if len(name) > 3 && name[len(name)-3:] == "Tag" {
		name = name[:len(name)-3]
	}
	return fmt.Sprintf("%s_id", toSnakeCase(name))
}

func toSnakeCase(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+32))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
