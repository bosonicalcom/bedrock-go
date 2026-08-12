package pagex

// A Page represents a subset of items from a larger data set, with pagination metadata for further iterations.
type Page[T any] struct {
	// Items list of items in the page.
	Items []T
	// TotalCount number of items in the whole data set.
	// Sometimes this value will not be computed to avoid serious performance hits.
	TotalCount int64
	// OffsetResult metadata for offset pagination.
	//
	// Sometimes this value will not be populated as it depends on the underlying storage implementation.
	OffsetResult *OffsetResult
	// TokenResult metadata for token pagination.
	//
	// Tokens are opaque strings that are used to paginate through the data set in an abstracted way.
	//
	// Sometimes this value will not be populated as it depends on the underlying storage implementation.
	TokenResult *TokenResult
}

// OffsetResult is a set of data that represents the result of an offset-based pagination query.
type OffsetResult struct {
	// PageNumber is the current page number.
	PageNumber int64
	// NextPageNumber is the page number of the next page (if available).
	NextPageNumber int64
	// PreviousPageNumber is the page number of the previous page (if available).
	PreviousPageNumber int64
}

// TokenResult is a set of data that represents the result of a token-based pagination query.
//
// Tokens are opaque strings that are used to paginate through the data set in an abstracted way.
// They cannot be decoded or inspected by the caller.
type TokenResult struct {
	// NextPageToken is the token for the next page (if available).
	NextPageToken string
	// PreviousPageToken is the token for the previous page (if available).
	PreviousPageToken string
}

// HasNextPage returns true if the page has a next page (either by offset or token).
func (p *Page[T]) HasNextPage() bool {
	if p == nil {
		return false
	} else if p.OffsetResult != nil {
		return p.OffsetResult.NextPageNumber != 0
	}
	return p.TokenResult != nil && p.TokenResult.NextPageToken != ""
}

// HasPreviousPage returns true if the page has a previous page (either by offset or token).
func (p *Page[T]) HasPreviousPage() bool {
	if p == nil {
		return false
	} else if p.OffsetResult != nil {
		return p.OffsetResult.PreviousPageNumber != 0
	}
	return p.TokenResult != nil && p.TokenResult.PreviousPageToken != ""
}

// NextPageToken returns the token for the next page (if available).
func (p *Page[T]) NextPageToken() string {
	if p == nil || p.TokenResult == nil {
		return ""
	}
	return p.TokenResult.NextPageToken
}

// PreviousPageToken returns the token for the previous page (if available).
func (p *Page[T]) PreviousPageToken() string {
	if p == nil || p.TokenResult == nil {
		return ""
	}
	return p.TokenResult.PreviousPageToken
}

// PageNumber returns the page number of the current page (if available).
func (p *Page[T]) PageNumber() int64 {
	if p == nil || p.OffsetResult == nil {
		return 0
	}
	return p.OffsetResult.PageNumber
}

// NextPageNumber returns the page number of the next page (if available).
func (p *Page[T]) NextPageNumber() int64 {
	if p == nil || p.OffsetResult == nil {
		return 0
	}
	return p.OffsetResult.NextPageNumber
}

// PreviousPageNumber returns the page number of the previous page (if available).
func (p *Page[T]) PreviousPageNumber() int64 {
	if p == nil || p.OffsetResult == nil {
		return 0
	}
	return p.OffsetResult.PreviousPageNumber
}
