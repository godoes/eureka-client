package eureka_client

import (
	"encoding/json"
	"hash/fnv"
	"sync"
)

var ShardCount = 32

// ConcurrentMap A "thread" safe map of type string:Anything.
// To avoid lock bottlenecks this map is dived to several (ShardCount) map shards.
type ConcurrentMap []*ConcurrentMapShared

// ConcurrentMapShared A "thread" safe string to anything map.
type ConcurrentMapShared struct {
	items        map[string]interface{}
	sync.RWMutex // Read Write mutex, guards access to internal map.
}

// NewConcurrentMap Creates a new concurrent map.
func NewConcurrentMap() ConcurrentMap {
	m := make(ConcurrentMap, ShardCount)
	for i := 0; i < ShardCount; i++ {
		m[i] = &ConcurrentMapShared{items: make(map[string]interface{})}
	}
	return m
}

// GetShard Returns shard under given key
func (m ConcurrentMap) GetShard(key string) *ConcurrentMapShared {
	hash := fnv.New32()
	_, _ = hash.Write([]byte(key))
	return m[hash.Sum32()%uint32(ShardCount)]
}

func (m ConcurrentMap) MSet(data map[string]interface{}) {
	for key, value := range data {
		shard := m.GetShard(key)
		shard.Lock()
		shard.items[key] = value
		shard.Unlock()
	}
}

// Set Sets the given value under the specified key.
func (m ConcurrentMap) Set(key string, value interface{}) {
	// Get map shard.
	shard := m.GetShard(key)
	shard.Lock()
	shard.items[key] = value
	shard.Unlock()
}

// UpsertCb Callback to return new element to be inserted into the map
// It is called while lock is held, therefore it MUST NOT
// try to access other keys in same map, as it can lead to deadlock since
// Go sync.RWLock is not reentrant
type UpsertCb func(exist bool, valueInMap interface{}, newValue interface{}) interface{}

// Upsert Insert or Update - updates existing element or inserts a new one using UpsertCb
func (m ConcurrentMap) Upsert(key string, value interface{}, cb UpsertCb) (res interface{}) {
	shard := m.GetShard(key)
	shard.Lock()
	v, ok := shard.items[key]
	res = cb(ok, v, value)
	shard.items[key] = res
	shard.Unlock()
	return res
}

// SetIfAbsent Sets the given value under the specified key if no value was associated with it.
func (m ConcurrentMap) SetIfAbsent(key string, value interface{}) bool {
	// Get map shard.
	shard := m.GetShard(key)
	shard.Lock()
	_, ok := shard.items[key]
	if !ok {
		shard.items[key] = value
	}
	shard.Unlock()
	return !ok
}

// Get Retrieves an element from map under given key.
func (m ConcurrentMap) Get(key string) (interface{}, bool) {
	// Get shard
	shard := m.GetShard(key)
	shard.RLock()
	// Get item from shard.
	val, ok := shard.items[key]
	shard.RUnlock()
	return val, ok
}

// Count Returns the number of elements within the map.
func (m ConcurrentMap) Count() int {
	count := 0
	for i := 0; i < ShardCount; i++ {
		shard := m[i]
		shard.RLock()
		count += len(shard.items)
		shard.RUnlock()
	}
	return count
}

// Has Looks up an item under specified key
func (m ConcurrentMap) Has(key string) bool {
	// Get shard
	shard := m.GetShard(key)
	shard.RLock()
	// See if element is within shard.
	_, ok := shard.items[key]
	shard.RUnlock()
	return ok
}

// Remove Removes an element from the map.
func (m ConcurrentMap) Remove(key string) {
	// Try to get shard.
	shard := m.GetShard(key)
	shard.Lock()
	delete(shard.items, key)
	shard.Unlock()
}

// Pop PopRemoves an element from the map and returns it
func (m ConcurrentMap) Pop(key string) (v interface{}, exists bool) {
	// Try to get shard.
	shard := m.GetShard(key)
	shard.Lock()
	v, exists = shard.items[key]
	delete(shard.items, key)
	shard.Unlock()
	return v, exists
}

// IsEmpty Checks if map is empty.
func (m ConcurrentMap) IsEmpty() bool {
	return m.Count() == 0
}

// Tuple Used by the Iter & IterBuffered functions to wrap two variables together over a channel,
type Tuple struct {
	Key string
	Val interface{}
}

// Iter Returns an iterator which could be used in a for range loop.
//
// Deprecated: using IterBuffered() will get a better performance
func (m ConcurrentMap) Iter() <-chan Tuple {
	channels := snapshot(m)
	ch := make(chan Tuple)
	go fanIn(channels, ch)
	return ch
}

// IterBuffered Returns a buffered iterator which could be used in a for range loop.
func (m ConcurrentMap) IterBuffered() <-chan Tuple {
	channels := snapshot(m)
	total := 0
	for _, c := range channels {
		total += cap(c)
	}
	ch := make(chan Tuple, total)
	go fanIn(channels, ch)
	return ch
}

// Returns an array of channels that contains elements in each shard,
// which likely takes a snapshot of `m`.
// It returns once the size of each buffered channel is determined,
// before all the channels are populated using goroutines.
func snapshot(m ConcurrentMap) (channels []chan Tuple) {
	channels = make([]chan Tuple, ShardCount)
	wg := sync.WaitGroup{}
	wg.Add(ShardCount)
	// Foreach shard.
	for index, shard := range m {
		go func(index int, shard *ConcurrentMapShared) {
			defer wg.Done()
			// Foreach key, value pair.
			shard.RLock()
			ch := make(chan Tuple, len(shard.items))
			channels[index] = ch
			for key, val := range shard.items {
				ch <- Tuple{key, val}
			}
			shard.RUnlock()
			close(ch)
		}(index, shard)
	}
	wg.Wait()
	return channels
}

// fanIn reads elements from channels `channels` into channel `out`
func fanIn(channels []chan Tuple, out chan Tuple) {
	wg := sync.WaitGroup{}
	wg.Add(len(channels))
	for _, ch := range channels {
		go func(c chan Tuple) {
			for t := range c {
				out <- t
			}
			wg.Done()
		}(ch)
	}
	wg.Wait()
	close(out)
}

// Items Returns all items as map[string]interface{}
func (m ConcurrentMap) Items() map[string]interface{} {
	tmp := make(map[string]interface{})
	for _, shard := range m {
		shard.RLock()
		for k, v := range shard.items {
			tmp[k] = v
		}
		shard.RUnlock()
	}
	return tmp
}

// IterCb Iterator callback,called for every key,value found in
// maps. RLock is held for all calls for a given shard
// therefore callback sess consistent view of a shard,
// but not across the shards
type IterCb func(key string, v interface{})

// IterCb Callback based iterator, cheapest way to read
// all elements in a map.
func (m ConcurrentMap) IterCb(fn IterCb) {
	for idx := range m {
		shard := m[idx]
		shard.RLock()
		for key, value := range shard.items {
			fn(key, value)
		}
		shard.RUnlock()
	}
}

// Keys Return all keys as []string
func (m ConcurrentMap) Keys() []string {
	count := m.Count()
	keys := make([]string, 0, count)
	for _, shard := range m {
		shard.RLock()
		for key := range shard.items {
			keys = append(keys, key)
		}
		shard.RUnlock()
	}
	return keys
}

// MarshalJSON Reviles ConcurrentMap "private" variables to json marshal.
func (m ConcurrentMap) MarshalJSON() ([]byte, error) {
	tmp := make(map[string]interface{})
	for _, shard := range m {
		shard.RLock()
		for k, v := range shard.items {
			tmp[k] = v
		}
		shard.RUnlock()
	}
	return json.Marshal(tmp)
}
