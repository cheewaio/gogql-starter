package store

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/cheewaio/gogql-starter/graph/model"
	"github.com/google/uuid"
)

// SortField defines a single column to sort by, along with the sort direction.
type SortField struct {
	Field string
	Asc   bool
}

// Cursor represents a decoded pagination cursor containing the sort values and
// ID of the anchor record. Used for keyset (cursor-based) pagination.
type Cursor struct {
	SortValues []string
	ID         string
}

// CursorDirection indicates whether a decoded cursor should page forward or backward.
type CursorDirection byte

const (
	CursorDirectionNext CursorDirection = iota
	CursorDirectionPrevious
)

var cursorMagic = []byte{'G', 'Q', 'C'}

const cursorVersion byte = 1

// QueryInput is the internal representation of pagination, filtering, sorting,
// and search parameters after parsing from the GraphQL input type. It is the
// bridge between the resolver layer and the store layer.
type QueryInput struct {
	PageNumber *int32
	PageSize   int32

	First  int32
	After  *Cursor
	Last   int32
	Before *Cursor

	Sort        []SortField
	Filters     []*model.FilterCriteria
	FilterLogic model.FilterLogic
	Search      *model.SearchInput
}

// EncodeCursor packs a next-page cursor into a compact binary representation
// and returns it as a base64url-encoded string.
//
// Versioned binary format:
// [magic:3B][version:1B][direction:1B][nVals:1B][len1:2B][val1:len1]...[UUID:16B]
//
// Legacy binary format, still accepted as NEXT:
// [nVals:1B][len1:2B][val1:len1]...[lenN:2B][valN:lenN][UUID:16B]
// - nVals is capped at 255 sort fields
// - each value length is a uint16 (max ~65KB per sort value)
// - UUID is always 16 bytes
func EncodeCursor(sortValues []string, id string) string {
	return EncodePageCursor(sortValues, id, CursorDirectionNext)
}

// EncodePageCursor creates an opaque cursor that carries its pagination direction.
func EncodePageCursor(sortValues []string, id string, direction CursorDirection) string {
	var buf bytes.Buffer
	buf.Write(cursorMagic)
	buf.WriteByte(cursorVersion)
	buf.WriteByte(byte(direction))
	if !writeCursorPayload(&buf, sortValues, id) {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func writeCursorPayload(buf *bytes.Buffer, sortValues []string, id string) bool {
	buf.WriteByte(byte(len(sortValues))) //nolint:gosec // max 255 sort fields
	for _, v := range sortValues {
		_ = binary.Write(buf, binary.BigEndian, uint16(len(v))) //nolint:gosec // max 65KB per sort value
		buf.WriteString(v)
	}
	uid, err := uuid.Parse(id)
	if err != nil {
		return false
	}
	buf.Write(uid[:])
	return true
}

// DecodeCursor reverses EncodeCursor, extracting the sort values and UUID from
// a base64url-encoded cursor string. Returns an error if the cursor is
// malformed or the UUID is invalid.
func DecodeCursor(cursor string) (sortValues []string, id string, err error) {
	sortValues, id, _, err = DecodePageCursor(cursor)
	return sortValues, id, err
}

// DecodePageCursor decodes an opaque cursor and returns its embedded direction.
func DecodePageCursor(cursor string) (sortValues []string, id string, direction CursorDirection, err error) {
	data, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return nil, "", CursorDirectionNext, fmt.Errorf("invalid cursor encoding: %w", err)
	}

	direction = CursorDirectionNext
	if bytes.HasPrefix(data, cursorMagic) {
		if len(data) < len(cursorMagic)+2 {
			return nil, "", CursorDirectionNext, fmt.Errorf("invalid cursor: missing version or direction")
		}
		version := data[len(cursorMagic)]
		if version != cursorVersion {
			return nil, "", CursorDirectionNext, fmt.Errorf("unsupported cursor version: %d", version)
		}
		direction = CursorDirection(data[len(cursorMagic)+1])
		if direction != CursorDirectionNext && direction != CursorDirectionPrevious {
			return nil, "", CursorDirectionNext, fmt.Errorf("invalid cursor direction: %d", direction)
		}
		data = data[len(cursorMagic)+2:]
	}

	sortValues, id, err = decodeCursorPayload(data)
	return sortValues, id, direction, err
}

func decodeCursorPayload(data []byte) (sortValues []string, id string, err error) {
	r := bytes.NewReader(data)

	n, err := r.ReadByte()
	if err != nil {
		return nil, "", fmt.Errorf("invalid cursor: %w", err)
	}

	sortValues = make([]string, n)
	for i := range sortValues {
		var l uint16
		if err := binary.Read(r, binary.BigEndian, &l); err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		v := make([]byte, l)
		if _, err := io.ReadFull(r, v); err != nil {
			return nil, "", fmt.Errorf("invalid cursor: %w", err)
		}
		sortValues[i] = string(v)
	}

	uidBytes := make([]byte, 16)
	if _, err := io.ReadFull(r, uidBytes); err != nil {
		return nil, "", fmt.Errorf("invalid cursor: %w", err)
	}
	uid, err := uuid.FromBytes(uidBytes)
	if err != nil {
		return nil, "", fmt.Errorf("invalid cursor: %w", err)
	}

	return sortValues, uid.String(), nil
}
