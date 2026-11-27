package cache

import (
	"container/heap"
	"sync"
	"time"
)

// expirationItem represents an item in the expiration heap
type expirationItem struct {
	key        string
	expiration time.Time
	index      int // position in heap (maintained by heap.Interface)
}

// expirationHeap is a min-heap of cache items ordered by expiration time
type expirationHeap struct {
	items []*expirationItem
	mu    sync.Mutex
	// Map for O(1) lookup of items by key
	itemMap map[string]*expirationItem
}

// newExpirationHeap creates a new expiration heap
func newExpirationHeap() *expirationHeap {
	return &expirationHeap{
		items:   make([]*expirationItem, 0),
		itemMap: make(map[string]*expirationItem),
	}
}

// Push adds an item to the heap (called by heap.Push)
func (h *expirationHeap) Push(x interface{}) {
	item := x.(*expirationItem)
	item.index = len(h.items)
	h.items = append(h.items, item)
	h.itemMap[item.key] = item
}

// Pop removes and returns the minimum item (called by heap.Pop)
func (h *expirationHeap) Pop() interface{} {
	old := h.items
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // mark as removed
	h.items = old[0 : n-1]
	delete(h.itemMap, item.key)
	return item
}

// Len returns the number of items in the heap
func (h *expirationHeap) Len() int {
	return len(h.items)
}

// Less reports whether item i should sort before item j
func (h *expirationHeap) Less(i, j int) bool {
	return h.items[i].expiration.Before(h.items[j].expiration)
}

// Swap swaps the items at indices i and j
func (h *expirationHeap) Swap(i, j int) {
	h.items[i], h.items[j] = h.items[j], h.items[i]
	h.items[i].index = i
	h.items[j].index = j
}

// push adds an item with thread-safety
func (h *expirationHeap) push(key string, expiration time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If item already exists, remove it first
	if existing, exists := h.itemMap[key]; exists {
		heap.Remove(h, existing.index)
	}

	item := &expirationItem{
		key:        key,
		expiration: expiration,
	}
	heap.Push(h, item)
}

// remove removes an item by key with thread-safety
func (h *expirationHeap) remove(key string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if item, exists := h.itemMap[key]; exists {
		heap.Remove(h, item.index)
	}
}

// popExpired removes and returns all expired items up to the given time
func (h *expirationHeap) popExpired(now time.Time) []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	expired := make([]string, 0)

	for len(h.items) > 0 && h.items[0].expiration.Before(now) {
		item := heap.Pop(h).(*expirationItem)
		expired = append(expired, item.key)
	}

	return expired
}

// update updates the expiration time for a key
func (h *expirationHeap) update(key string, expiration time.Time) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if item, exists := h.itemMap[key]; exists {
		item.expiration = expiration
		heap.Fix(h, item.index)
	}
}

// clear removes all items from the heap
func (h *expirationHeap) clear() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.items = make([]*expirationItem, 0)
	h.itemMap = make(map[string]*expirationItem)
}
