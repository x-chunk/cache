# Design

How the two caches work inside, and why. The package docs (`go doc -all`)
describe the behavior you can rely on; this file explains the mechanisms
behind it. Constants are named so you can find them in the code.

## v1: `go.xchunk.org/cache`

One `map[K]Item[V]` behind one `sync.RWMutex`.

- **Get** takes the read lock, looks the key up and compares the item's
  `expiresAt` (a `time.Time`, zero for "never") with `time.Now()`. Concurrent
  Gets don't block each other, but they all update the same reader count, so
  under heavy parallel reads they contend on one cache line.
- **Set** takes the write lock. If the next sweep is due, it scans the whole
  map and deletes expired items, then schedules the next sweep in
  `max(ttl, minSweepInterval)` (one minute). The interval keeps a very short
  ttl from making every Set scan the whole map.
- **Cleanup** runs the same full scan on demand; **RunCleanup** calls it on a
  ticker.

The clock is a `now func() time.Time` field so tests can replace it.

## v2: `go.xchunk.org/cache/v2`

v2 keeps v1's semantics but changes three things: how time is read, how the
map is locked, and how the size can be bounded.

### Coarse clock

Calling `time.Now` on every Get costs tens of nanoseconds, more on some
virtual machines, and was most of the cost of a v1 Get hit. v2 checks expiry
against a `Clock` instead:

```go
type Clock interface {
	Nanotime() int64 // monotonic nanoseconds; only differences matter
}
```

The default is a process-wide `CoarseClock`: a goroutine stores
`time.Since(base)` into an `atomic.Int64` every `DefaultTick` (1ms), and
`Nanotime` just loads it. The goroutine starts with the first `New` and runs
until the process exits; it is shared by every cache, so creating caches
never leaks goroutines and needs no `Close`.

Consequences:

- The clock lags real time by up to a tick, plus scheduler and GC delays.
  Items therefore never expire early, but may be returned for about a tick
  after their ttl.
- Items store expiry as an `int64` on the clock's scale instead of a
  `time.Time`: 8 bytes instead of 24, which keeps the map smaller.
- Get reads the clock only for items that have a ttl.
- `addNanos` saturates at `math.MaxInt64` instead of overflowing, so a huge
  ttl means "never expires" rather than "already expired".
- `WithClock` swaps in another clock: a `NewCoarseClock` with a different
  tick (which you `Stop` yourself), a precise clock built on `time.Now`, or a
  fake clock in tests. If a clock stops, items stop expiring.

### Shards

Even a read lock writes to shared memory, so with one lock goroutines slow
each other down however few writes there are. v2 splits the map into
independent shards, each a map with its own `RWMutex`:

- A key's shard is `maphash.Comparable(seed, key) & mask`. The shard count
  is a power of two, so the mask replaces a modulo. With a single shard the
  hash is skipped.
- Each `shard` struct ends with 64 bytes of padding so that neighboring
  shards' locks don't share a CPU cache line.
- The default count is `shardsPerProc` (64) times `GOMAXPROCS`, rounded up
  to a power of two and capped at `maxDefaultShards` (1024). Lock contention
  falls roughly as 1/shards; past about 64 shards per core Get stops
  improving, while writes keep improving up to 1024.
- Hashing costs about 5ns per operation, so a cache used by one goroutine is
  slightly faster with `WithShards(1)`. An empty shard costs about 155 bytes,
  which matters when you create many small caches.

The measurements behind these numbers are in the `WithShards` doc comment and
in `v2/bench_test.go`.

### Sweeping expired items

A full sweep under a shard lock would make some Sets slow on a large shard.
v2 sweeps incrementally instead:

- When a shard's sweep is due, Set examines at most `sweepBatch` (256)
  items of that shard and deletes the expired ones. Go randomizes where map
  iteration starts, so successive batches look at different items.
- If at least 1/`sweepAgainRatio` (a quarter) of a full batch was expired,
  the shard probably holds many more, and the next Set to that shard sweeps
  again. Otherwise the next sweep is scheduled `max(ttl, one minute)` later.
- This is probabilistic: a few expired items may stay until overwritten,
  invalidated or removed by `Cleanup`, which still scans every shard fully
  (one shard lock at a time).

### Bounded size (`WithMaxEntries`)

Without a bound, a cache keyed by untrusted input can grow without limit.
`WithMaxEntries(n)` caps it:

- The cap is per shard: each shard holds at most `n / shards` items, so the
  whole cache never exceeds `n`. With an uneven key spread, a shard may evict
  before the cache is full.
- To keep shards from being too small, `New` halves the shard count until
  each shard holds at least `minBoundedShardCap` (64) items. A cache bounded
  to fewer than 128 items has a single shard.
- Set checks `len(shard) >= cap` before looking up the key, so Sets to a
  shard that isn't full pay nothing. Overwriting an existing key never
  evicts.
- When a new key goes into a full shard, `evict` removes a batch of items.
  Starting a map iteration costs about as much as visiting three or four
  items, so evicting in batches is cheaper than one at a time. The batch is
  `max(1, min(maxEvictBatch, cap/evictBatchDivisor))`: up to 8 items, but no
  more than 1/16 of the shard.
- `evict` samples `evictSamplePerItem` (4) random items per item to remove.
  It deletes every expired item in the sample, then the live sampled items
  that expire soonest (items without a ttl count as expiring last). The
  sample is at most 32 items and lives on the stack, so eviction doesn't
  allocate.

This is an approximation of "soonest expiry first", not LRU. Reads don't
affect eviction, which keeps Get free of writes. With one ttl for all items it
behaves close to evicting the oldest item.

## Shared loads (`GetOrSet`)

Both versions implement `GetOrSet` the same way; v2 builds it on
`GetOrSetContext`.

1. Try `Get`. A hit returns without touching any other lock.
2. Under `callsMu`, look for an in-flight load of the key in `calls`. If
   there is one, wait on its `done` channel (in v2, or on `ctx.Done()`).
3. Otherwise register a new call and become its leader. Check `Get` once
   more, because another leader may have stored the value in between.
4. Run `load`. On success, `Set` the value. Then remove the call from
   `calls` and close `done`, releasing the waiters with the leader's value
   and error.

Errors are returned to every waiter and are not cached, so the next call
retries. If `load` panics, a deferred function marks the call as failed with
the loader-panicked error (`ErrLoaderPanicked` in v2), cleans up and wakes the
waiters; the panic itself continues in the leader's goroutine.

In v2, a waiter whose context ends returns `ctx.Err()` right away. The load
it was waiting for keeps running and its result is still cached. The leader
passes its own context to `load`, so if that context is canceled and `load`
returns the error, the waiters get the same error even if their contexts
are alive; they can retry.

`callsMu` is one mutex for the whole cache, not per shard. It is taken only
on a miss, which is already slower than a hit because it runs `load`.
