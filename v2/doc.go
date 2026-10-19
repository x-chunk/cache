// Package cache provides an in-memory TTL cache that is safe for concurrent
// use.
//
// It has the API of go.xchunk.org/cache (v1), plus options for New
// (WithShards, WithMaxEntries, WithClock) and GetOrSetContext. Unlike v1, it
// checks expiry against a coarse clock instead of calling time.Now on every
// Get. That makes Get of an item with a ttl several times faster, at the cost
// of precision: an item may outlive its ttl by up to the clock's tick
// (DefaultTick, 1ms, for the shared default clock) plus scheduler and GC
// delays. Items never expire early. If you need exact expiry for very short
// ttls, use go.xchunk.org/cache or pass a precise Clock with WithClock.
//
// By default all caches share one process-wide clock driven by a single
// goroutine that is started with the first cache and runs until the process
// exits. No Close is needed and nothing leaks however many caches are
// created. See CoarseClock and WithClock for the details and alternatives.
//
// # Stability
//
// This module is unstable. Until v2.0.0 is released, its versions are
// v2.0.0 pre-releases (such as v2.0.0-alpha.1) and its API may change in any
// of them. go.xchunk.org/cache (v1) is the stable version.
//
// # Performance
//
// BenchmarkGetCompare and BenchmarkGetCompareParallel in
// go.xchunk.org/cache (file get_bench_test.go) compare Get of the v1
// cache, the same Get without any expiry check, v1 with the clock read only
// for expiring items, and this package's coarse-clock approach. Mean ns/op
// over 5 runs on a 4-core cloud VM (Go 1.27, Intel Xeon 2.1GHz), where
// time.Now is unusually slow; on bare metal the gap to v1 is smaller:
//
//	Scenario             v1 Get  no check  v1 lazy clock  coarse clock
//	hit, item has ttl     103.3      28.0          110.4          28.5
//	hit, no ttl           104.3      28.1           26.5          27.8
//	miss                   28.1      26.2           25.3          26.4
//	hit, ttl, 4 gorout.   134.2     112.9          123.2         111.0
//	hit, no ttl, 4 gor.   107.4     113.9          105.2         112.9
//	miss, 4 goroutines    110.1     109.0          109.2         110.4
//
// With 4 goroutines all variants are bound by the RWMutex reader count, and
// differences of about 5ns are noise. Sharding (see WithShards) was added to
// address that contention; the numbers above were measured before it, with a
// single lock. Reproduce from the repository root with:
//
//	go test . -run '^$' -bench GetCompare -benchmem -count 5 -cpu 4
package cache
