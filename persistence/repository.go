package persistence

//go:generate go tool mockgen -source=repository.go -destination=persistencetest/repository_mock.go -package=persistencetest

import (
	"context"
	"errors"
)

var (
	ErrEntryNotFound = errors.New("entry not found")
)

type Repository[K comparable, T any] interface {
	WriteRepository[K, T]
	ReadRepository[K, T]
}

type WriteRepository[K comparable, T any] interface {
	Save(ctx context.Context, v *T) error
	Delete(ctx context.Context, v *T) error
	DeleteByKey(ctx context.Context, key K) error
}

type WriteBatchRepository[K comparable, T any] interface {
	// SaveAll persists the given set of values.
	//
	// Returns all keys of the persisted values.
	SaveAll(ctx context.Context, v ...T) ([]K, error)
	// DeleteAll deletes the given set of values from the persistent storage.
	//
	// Returns all keys of the persisted values.
	DeleteAll(ctx context.Context, v ...T) ([]K, error)
	// DeleteByKeys deletes all values with the given set of keys from the persistence storage.
	//
	// Returns all keys of the persisted values.
	DeleteByKeys(ctx context.Context, keys ...K) ([]K, error)
}

type ReadRepository[K comparable, T any] interface {
	GetByKey(ctx context.Context, key K) (*T, error)
}
