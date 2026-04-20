package pagination

import (
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

type Params struct {
	Limit  int
	Cursor string
}

func Parse(limitStr, cursorStr string) Params {
	limit := DefaultLimit
	if n, err := strconv.Atoi(limitStr); err == nil {
		switch {
		case n <= 0:
			limit = DefaultLimit
		case n > MaxLimit:
			limit = MaxLimit
		default:
			limit = n
		}
	}

	return Params{
		Limit:  limit,
		Cursor: DecodeCursor(cursorStr),
	}
}

func EncodeCursor(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func DecodeCursor(encoded string) string {
	if encoded == "" {
		return ""
	}

	b, err := base64.RawURLEncoding.DecodeString(encoded)

	// treat invalid cursor as first page
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(b))
}

func NewCursor(createdAt time.Time, id uuid.UUID) string {
	internal := fmt.Sprintf("%s:%s", createdAt.UTC().Format(time.RFC3339Nano), id.String())

	return EncodeCursor(internal)
}

func ParseCursor(decoded string) (time.Time, uuid.UUID, bool) {
	if decoded == "" {
		return time.Time{}, uuid.Nil, false
	}

	lastColon := strings.LastIndex(decoded, ":")
	if lastColon == -1 {
		return time.Time{}, uuid.Nil, false
	}

	timeStr := decoded[:lastColon]
	idStr := decoded[lastColon+1:]

	t, err := time.Parse(time.RFC3339Nano, timeStr)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		return time.Time{}, uuid.Nil, false
	}

	return t, id, true
}

type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"next_cursor,omitempty"`
	HasMore    bool   `json:"has_more"`
}

func NewPage[T any](items []T, limit int, cursorFn func(T) string) Page[T] {
	if len(items) == 0 {
		return Page[T]{Items: []T{}, HasMore: false}
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit] // trim the sentinel item
	}

	page := Page[T]{
		Items:   items,
		HasMore: hasMore,
	}

	if hasMore {
		last := items[len(items)-1]
		page.NextCursor = cursorFn(last)
	}

	return page
}
