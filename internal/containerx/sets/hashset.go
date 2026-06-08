package sets

// HashSet is a basic hash [Set] implementation in Go.
type HashSet[T comparable] map[T]struct{}

var _ Set[string] = (*HashSet[string])(nil)

// NewHashSet creates a new [HashSet] with the provided items.
func NewHashSet[T comparable](items ...T) HashSet[T] {
	h := HashSet[T]{}
	for _, i := range items {
		h.Add(i)
	}
	return h
}

func (h HashSet[T]) Add(item T) bool {
	_, found := h[item]
	if found {
		return false
	}
	h[item] = struct{}{}
	return true
}

func (h HashSet[T]) Remove(item T) bool {
	_, found := h[item]
	if !found {
		return false
	}
	delete(h, item)
	return true
}

func (h HashSet[T]) Contains(item T) bool {
	_, found := h[item]
	return found
}

func (h HashSet[T]) Clear() {
	clear(h)
}

func (h HashSet[T]) Len() int {
	return len(h)
}
