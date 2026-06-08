package sets

// Set represents a generic set interface that can hold items of any comparable type.
type Set[T comparable] interface {
	// Add adds an item to the set. Returns true if the item was added, false if it was already present.
	Add(item T) bool
	// Remove removes an item from the set. Returns true if the item was removed, false if it was not present.
	Remove(item T) bool
	// Contains checks if an item is in the set. Returns true if the item is present, false if it is not.
	Contains(item T) bool
	// Len returns the number of items in the set.
	Len() int
	// Clear removes all items from the set.
	Clear()
}
