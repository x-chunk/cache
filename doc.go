// Package cache provides an in-memory key-value cache whose items expire
// after a time-to-live (ttl). It is generic over the key and value types and
// safe for concurrent use.
//
//	c := cache.New[string, User](time.Minute)
//	c.Set("alice", alice)
//	u, ok := c.Get("alice")
//
// # Expiry
//
// Every item lives for the cache's default ttl, set by New, or for its own
// ttl, set by SetWithTTL. A ttl of zero or less means the item never
// expires. Expiry is checked against time.Now on every Get, so an item is
// never returned after its ttl has passed.
//
// Expired items are not removed on Get. Set sweeps the whole cache for them
// at most once per max(ttl, one minute), and Cleanup or RunCleanup remove
// them on demand. Call RunCleanup in its own goroutine if the cache is
// written to rarely but must not hold on to expired items.
//
// # Loading values
//
// GetOrSet returns the cached value or calls a load function to produce it.
// Concurrent GetOrSet calls for the same missing key share one load: one
// caller runs it and the others wait for its result. Errors from load are
// returned to every waiting caller and are not cached.
//
// # Choosing between v1 and v2
//
// This package (v1) is the simple, precise version: one map behind one
// RWMutex, with exact expiry. go.xchunk.org/cache/v2 has the same core API
// but reads a shared coarse clock instead of calling time.Now, splits the
// map into shards to reduce lock contention, and can bound its size with
// WithMaxEntries. Use v2 for hot caches read by many goroutines or for
// caches whose keys come from untrusted input; use v1 when exact expiry
// matters more than speed. v2 is not yet stable.
package cache
