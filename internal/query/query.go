package query

// Pagination holds limit/offset for paginated queries.
// Transport-agnostic — no HTTP or framework imports.
type Pagination struct {
	Limit  int
	Offset int
}

// DefaultLimit is applied when the caller sends zero or negative.
const DefaultLimit = 20

// MaxLimit caps runaway requests.
const MaxLimit = 100

// Normalize clamps Limit to [1, MaxLimit] and Offset to >= 0.
func (p Pagination) Normalize() Pagination {
	if p.Limit <= 0 {
		p.Limit = DefaultLimit
	}
	if p.Limit > MaxLimit {
		p.Limit = MaxLimit
	}
	if p.Offset < 0 {
		p.Offset = 0
	}
	return p
}

// Page carries pagination metadata in API responses — the effective
// limit/offset the server applied (after clamping) plus the total match count.
type Page struct {
	Total  int `json:"total"`
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// Paginated is the standard paginated response envelope.
// Wire format: {"items": [...], "page": {"total": N, "limit": N, "offset": N}}
type Paginated[T any] struct {
	Items []T  `json:"items"`
	Page  Page `json:"page"`
}
