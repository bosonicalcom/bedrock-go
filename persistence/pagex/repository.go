package pagex

import (
	"context"
)

//go:generate go tool mockgen -source=repository.go -destination=pagextest/repository_mock.go -package=pagextest

const (
	_defaultPageSizeOpt int32 = 25
)

// PageRepository is a persistence component that fetches dataset items in pages.
type PageRepository[T any] interface {
	// List retrieves a [Page] ot T.
	//
	// Use [ListOption] to customize how many items to fetch, indicate next page, etc.
	List(ctx context.Context, opts ...ListOption) (*Page[*T], error)
}

type ListOptions struct {
	PageSize   int32
	PageNumber int64
	PageToken  string
}

// SafePageSize returns a page size where N will always be a positive integer.
func (l ListOptions) SafePageSize() int32 {
	if l.PageSize <= 0 {
		return _defaultPageSizeOpt
	}
	return l.PageSize
}

type ListOption func(options *ListOptions)

// NewDefaultListOptions returns a default ListOptions instance with a page size of 25.
func NewDefaultListOptions() *ListOptions {
	return &ListOptions{
		PageSize: 25,
	}
}

// WithPageSize sets the maximum number of items to be retrieved by a List operation.
func WithPageSize(n int32) ListOption {
	return func(options *ListOptions) {
		options.PageSize = n
	}
}

// WithPageNumber sets the page number to be retrieved by a List operation.
func WithPageNumber(n int64) ListOption {
	return func(options *ListOptions) {
		options.PageNumber = n
	}
}

// WithPageToken sets the cursor where the List operation should start/continue fetching.
func WithPageToken(v string) ListOption {
	return func(options *ListOptions) {
		options.PageToken = v
	}
}
