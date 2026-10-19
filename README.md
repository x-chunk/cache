# cache

Generic in-memory TTL caches for Go, safe for concurrent use. No
dependencies outside the standard library.

This repository holds two modules:

| Module | Status | What it is |
|---|---|---|
| [`go.xchunk.org/cache`](.) | Stable | One map behind one `RWMutex`. Expiry is checked against `time.Now`, so it is exact. |
| [`go.xchunk.org/cache/v2`](v2) | **Unstable** | Same core API. Checks expiry against a shared coarse clock (refreshed every 1ms), splits the map into shards, and can bound its size. `Get` is several times faster; items may outlive their ttl by about a tick. |

Both packages are named `cache`. Both require Go 1.27 or newer.

The code used to live in
[`go.xchunk.org/anvil/v2`](https://github.com/x-chunk/anvil) as `cache` and
`cache/v2`. See [docs/migrating-from-anvil.md](docs/migrating-from-anvil.md).

## Install

```sh
go get go.xchunk.org/cache       # v1
go get go.xchunk.org/cache/v2    # v2
```

## Usage

```go
import "go.xchunk.org/cache"

c := cache.New[string, User](time.Minute) // items live for one minute

c.Set("alice", alice)
u, ok := c.Get("alice")                    // ok is false if missing or expired

c.SetWithTTL("session", s, 10*time.Second) // a ttl of its own
c.SetWithTTL("config", cfg, 0)             // 0 or less: never expires

c.Invalidate("alice")
```

`GetOrSet` returns the cached value or loads it. Concurrent calls for the
same missing key share one load, and errors are not cached:

```go
u, err := c.GetOrSet("alice", func() (User, error) {
	return db.LoadUser(ctx, "alice") // runs once, however many goroutines ask
})
```

Expired items are never returned. Their memory is reclaimed by later `Set`
calls, which sweep periodically, or on demand:

```go
removed := c.Cleanup()            // once
go c.RunCleanup(ctx, time.Minute) // in the background, until ctx is done
```

### v2

v2 has the same methods plus options for `New` and a cancelable
`GetOrSetContext`:

```go
import "go.xchunk.org/cache/v2"

c := cache.New[string, User](time.Minute,
	cache.WithMaxEntries(100_000), // bound the size; evicts expired, then soonest to expire
	cache.WithShards(256),         // default: 64 per GOMAXPROCS, up to 1024
)

u, err := c.GetOrSetContext(ctx, "alice", func(ctx context.Context) (User, error) {
	return db.LoadUser(ctx, "alice")
})
if errors.Is(err, cache.ErrLoaderPanicked) {
	// another goroutine's load panicked; safe to retry
}
```

`WithClock` replaces the shared coarse clock, for example with a
`NewCoarseClock` of a different tick or a fake clock in tests.

## Which one to use

- **v1** when exact expiry matters, the cache is small or mostly used by one
  goroutine, or you need a stable API.
- **v2** for hot caches read by many goroutines, and for caches whose keys
  come from untrusted input (use `WithMaxEntries` so they can't grow without
  limit).

Benchmarks and the reasoning behind the defaults are in the
[v2 package docs](v2/doc.go) and in [docs/design.md](docs/design.md).

## Documentation

- Package docs: `go doc -all go.xchunk.org/cache` and
  `go doc -all go.xchunk.org/cache/v2`, or pkg.go.dev.
- Runnable examples: `example_test.go` in each module.
- How the caches work inside: [docs/design.md](docs/design.md).

## Versioning

Both modules follow [semantic versioning](https://semver.org). The root
module is released with `v1.x.y` tags. The v2 module lives in the `v2`
directory and is released with `v2.x.y` tags; until it is declared stable,
its versions are `v2.0.0` pre-releases (such as `v2.0.0-alpha.1`) and its
API may change in any of them.

## License

[GPL-3.0](LICENSE).
