package health

import "context"

// A Monitor represents a health check that can be performed on a component/resource (e.g., filesystem usage, database reachable)
type Monitor interface {
	Name() string
	Check(ctx context.Context) error
}
