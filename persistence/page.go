package persistence

type Page[T any] struct {
	Items        []T
	TotalCount   int64
	OffsetResult *PageOffsetResult
	CursorResult *PageCursorResult
}

type PageOffsetResult struct {
	PageNumber int64
	PageSize   int64
	// TotalCount number of items in the whole data set.
	// Sometimes this value will not be computed to avoid serious performance hits.
	TotalCount         int64
	NextPageNumber     int64
	PreviousPageNumber int64
}

type PageCursorResult struct {
	NextPageToken     string
	PreviousPageToken string
}
