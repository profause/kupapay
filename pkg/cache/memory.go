package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type lruNode struct {
	key  string
	prev *lruNode
	next *lruNode
}

type lruList struct {
	head *lruNode
	tail *lruNode
	size int
}

func newLRUList() *lruList {
	return &lruList{}
}

func (l *lruList) moveToFront(node *lruNode) {
	if node == nil || node == l.head {
		return
	}

	if node.prev != nil {
		node.prev.next = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	}
	if node == l.tail {
		l.tail = node.prev
		if l.tail != nil {
			l.tail.next = nil
		}
	}

	node.prev = nil
	node.next = l.head
	if l.head != nil {
		l.head.prev = node
	}
	l.head = node
	if l.tail == nil {
		l.tail = node
	}
}

func (l *lruList) addToFront(node *lruNode) {
	node.prev = nil
	node.next = l.head
	if l.head != nil {
		l.head.prev = node
	}
	l.head = node
	if l.tail == nil {
		l.tail = node
	}
	l.size++
}

func (l *lruList) removeTail() *lruNode {
	if l.tail == nil {
		return nil
	}

	node := l.tail
	if node.prev != nil {
		node.prev.next = nil
	}
	l.tail = node.prev
	if l.head == node {
		l.head = nil
	}
	l.size--
	return node
}

func (l *lruList) remove(node *lruNode) {
	if node == nil {
		return
	}
	if node.prev != nil {
		node.prev.next = node.next
	} else {
		l.head = node.next
	}
	if node.next != nil {
		node.next.prev = node.prev
	} else {
		l.tail = node.prev
	}
	l.size--
}

type MemoryCacheItem struct {
	Value      string
	Expiration time.Time
	lruNode    *lruNode
}

type MemoryCache struct {
	mu              sync.RWMutex
	items           map[string]*MemoryCacheItem
	lru             *lruList
	expirationHeap  *expirationHeap
	maxSize         int
	cleanupInterval time.Duration
	defaultTTL      time.Duration
	stopCleanup     chan struct{}
	closeOnce       sync.Once
	stats           MemoryCacheStats
	onEvict         func(key string, value string)
}

type MemoryCacheStats struct {
	Hits      atomic.Int64
	Misses    atomic.Int64
	Evictions atomic.Int64
	Sets      atomic.Int64
	Deletes   atomic.Int64
}

type StatsSnapshot struct {
	Hits             int64
	Misses           int64
	Evictions        int64
	Sets             int64
	Deletes          int64
	HitRate          float64
	ItemCount        int
	CurrentMemory    int64
	MemoryEvictions  int64
	ExpiredEvictions int64
	LRUEvictions     int64
}

type MemoryCacheConfig struct {
	MaxSize         int
	CleanupInterval time.Duration
	DefaultTTL      time.Duration
	OnEvict         func(key string, value string)
}

const (
	DefaultMaxSize    = 1000
	DefaultCleanupInt = 60 * time.Second
	DefaultTTLValue   = 5 * time.Minute
)

func NewMemoryCache(config MemoryCacheConfig) (*MemoryCache, error) {
	if config.MaxSize <= 0 {
		config.MaxSize = DefaultMaxSize
	}
	if config.CleanupInterval <= 0 {
		config.CleanupInterval = DefaultCleanupInt
	}
	if config.DefaultTTL <= 0 {
		config.DefaultTTL = DefaultTTLValue
	}

	mc := &MemoryCache{
		items:           make(map[string]*MemoryCacheItem),
		lru:             newLRUList(),
		expirationHeap:  newExpirationHeap(),
		maxSize:         config.MaxSize,
		cleanupInterval: config.CleanupInterval,
		defaultTTL:      config.DefaultTTL,
		stopCleanup:     make(chan struct{}),
		onEvict:         config.OnEvict,
	}
	go mc.cleanupLoop()

	return mc, nil
}

func (mc *MemoryCache) Get(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", WithKey(ErrInvalidKey, key)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	return mc.getLocked(key, 0)
}

func (mc *MemoryCache) GetDel(ctx context.Context, key string) (string, error) {
	if key == "" {
		return "", WithKey(ErrInvalidKey, key)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	value, err := mc.getLocked(key, 0)
	if err != nil {
		return "", err
	}
	if item, ok := mc.items[key]; ok {
		mc.removeItemLocked(key, item, true)
	}
	return value, nil
}

func (mc *MemoryCache) GetJSON(ctx context.Context, key string, dest any) error {
	if dest == nil {
		return fmt.Errorf("destination cannot be nil")
	}

	data, err := mc.Get(ctx, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return WithKey(ErrDeserializationFailed.Wrap(err), key)
	}
	return nil
}

func (mc *MemoryCache) GetJSONWithRefresh(ctx context.Context, key string, dest any, extendTTL time.Duration) error {
	if key == "" {
		return WithKey(ErrInvalidKey, key)
	}
	if dest == nil {
		return fmt.Errorf("destination cannot be nil")
	}

	mc.mu.Lock()
	data, err := mc.getLocked(key, extendTTL)
	mc.mu.Unlock()
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(data), dest); err != nil {
		return WithKey(ErrDeserializationFailed.Wrap(err), key)
	}
	return nil
}

func (mc *MemoryCache) Set(ctx context.Context, key string, value string, expiration time.Duration) error {
	if key == "" {
		return WithKey(ErrInvalidKey, key)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.setLocked(key, value, expiration)
	return nil
}

func (mc *MemoryCache) SetJSON(ctx context.Context, key string, value any, expiration time.Duration) error {
	if value == nil {
		return WithKey(ErrNilValue, key)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return WithKey(ErrSerializationFailed.Wrap(err), key)
	}
	return mc.Set(ctx, key, string(payload), expiration)
}

func (mc *MemoryCache) Delete(ctx context.Context, keys ...string) error {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	for _, key := range keys {
		if item, ok := mc.items[key]; ok {
			mc.removeItemLocked(key, item, true)
		}
	}

	return nil
}

// DeleteByPattern removes items by prefix. Only trailing-"*" patterns are
// honored ("foo:*"); anything else is treated as a literal key delete (this
// matches every current caller — settings deletes "settings:*"). For Redis
// the Redis backend supports general glob; we mirror what's actually used to
// avoid implementing a glob matcher we don't need.
func (mc *MemoryCache) DeleteByPattern(ctx context.Context, pattern string) error {
	if pattern == "" {
		return nil
	}
	if !strings.HasSuffix(pattern, "*") {
		return mc.Delete(ctx, pattern)
	}
	prefix := strings.TrimSuffix(pattern, "*")

	mc.mu.Lock()
	defer mc.mu.Unlock()

	for key, item := range mc.items {
		if strings.HasPrefix(key, prefix) {
			mc.removeItemLocked(key, item, true)
		}
	}
	return nil
}

func (mc *MemoryCache) SetIfAbsent(ctx context.Context, key string, value string, expiration time.Duration) (bool, error) {
	if key == "" {
		return false, WithKey(ErrInvalidKey, key)
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	if item, ok := mc.items[key]; ok {
		if time.Now().Before(item.Expiration) {
			return false, nil
		}
		mc.removeItemLocked(key, item, false)
	}

	mc.setLocked(key, value, expiration)
	return true, nil
}

func (mc *MemoryCache) Clear() {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	mc.items = make(map[string]*MemoryCacheItem)
	mc.lru = newLRUList()
	mc.expirationHeap.clear()
}

func (mc *MemoryCache) Size() int {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	return len(mc.items)
}

func (mc *MemoryCache) GetStatsSnapshot() StatsSnapshot {
	hits := mc.stats.Hits.Load()
	misses := mc.stats.Misses.Load()

	var hitRate float64
	if hits+misses > 0 {
		hitRate = float64(hits) / float64(hits+misses)
	}

	return StatsSnapshot{
		Hits:      hits,
		Misses:    misses,
		Evictions: mc.stats.Evictions.Load(),
		Sets:      mc.stats.Sets.Load(),
		Deletes:   mc.stats.Deletes.Load(),
		HitRate:   hitRate,
		ItemCount: mc.Size(),
	}
}

func (mc *MemoryCache) Close() error {
	mc.closeOnce.Do(func() {
		close(mc.stopCleanup)
	})
	return nil
}

func (mc *MemoryCache) getLocked(key string, extendTTL time.Duration) (string, error) {
	item, ok := mc.items[key]
	if !ok {
		mc.stats.Misses.Add(1)
		return "", WithKey(ErrKeyNotFound, key)
	}

	now := time.Now()
	if !item.Expiration.IsZero() && now.After(item.Expiration) {
		mc.removeItemLocked(key, item, false)
		mc.stats.Misses.Add(1)
		return "", WithKey(ErrKeyExpired, key)
	}

	if extendTTL > 0 {
		item.Expiration = now.Add(extendTTL)
		mc.expirationHeap.update(key, item.Expiration)
	}

	if item.lruNode != nil {
		mc.lru.moveToFront(item.lruNode)
	}

	mc.stats.Hits.Add(1)
	return item.Value, nil
}

func (mc *MemoryCache) setLocked(key string, value string, expiration time.Duration) {
	if expiration <= 0 {
		expiration = mc.defaultTTL
	}

	newExpiration := time.Now().Add(expiration)
	if existing, ok := mc.items[key]; ok {
		existing.Value = value
		existing.Expiration = newExpiration
		mc.expirationHeap.update(key, newExpiration)
		if existing.lruNode != nil {
			mc.lru.moveToFront(existing.lruNode)
		}
		mc.stats.Sets.Add(1)
		return
	}

	if len(mc.items) >= mc.maxSize {
		mc.evictLRU()
	}

	node := &lruNode{key: key}
	mc.lru.addToFront(node)
	mc.items[key] = &MemoryCacheItem{
		Value:      value,
		Expiration: newExpiration,
		lruNode:    node,
	}
	mc.expirationHeap.push(key, newExpiration)
	mc.stats.Sets.Add(1)
}

func (mc *MemoryCache) removeItemLocked(key string, item *MemoryCacheItem, deleted bool) {
	if item.lruNode != nil {
		mc.lru.remove(item.lruNode)
	}
	mc.expirationHeap.remove(key)
	delete(mc.items, key)

	if deleted {
		mc.stats.Deletes.Add(1)
	} else {
		mc.stats.Evictions.Add(1)
	}

	if mc.onEvict != nil {
		mc.onEvict(key, item.Value)
	}
}

func (mc *MemoryCache) evictLRU() {
	node := mc.lru.removeTail()
	if node == nil {
		return
	}

	item, ok := mc.items[node.key]
	if !ok {
		return
	}

	mc.expirationHeap.remove(node.key)
	delete(mc.items, node.key)
	mc.stats.Evictions.Add(1)

	if mc.onEvict != nil {
		mc.onEvict(node.key, item.Value)
	}
}

func (mc *MemoryCache) cleanupLoop() {
	ticker := time.NewTicker(mc.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-mc.stopCleanup:
			return
		case <-ticker.C:
			mc.deleteExpired()
		}
	}
}

func (mc *MemoryCache) deleteExpired() {
	expiredKeys := mc.expirationHeap.popExpired(time.Now())
	if len(expiredKeys) == 0 {
		return
	}

	mc.mu.Lock()
	defer mc.mu.Unlock()

	for _, key := range expiredKeys {
		if item, ok := mc.items[key]; ok {
			if item.lruNode != nil {
				mc.lru.remove(item.lruNode)
			}
			delete(mc.items, key)
			mc.stats.Evictions.Add(1)

			if mc.onEvict != nil {
				mc.onEvict(key, item.Value)
			}
		}
	}
}
