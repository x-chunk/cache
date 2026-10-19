# AGENTS.md

Instructions for coding agents that change this repository. Humans are
welcome too.

## What this is

Generic in-memory TTL caches for Go, in two modules:

- `go.xchunk.org/cache` (repository root): v1, stable. One map, one
  `RWMutex`, exact expiry against `time.Now`.
- `go.xchunk.org/cache/v2` (`v2/`): v2, **unstable**. Same core API plus
  `New` options and `GetOrSetContext`. Coarse shared clock, sharded map,
  optional size bound.

Both moved here from `go.xchunk.org/anvil/v2` (anvil commit `a136340`).
How they work inside is in [`docs/design.md`](docs/design.md); keep it in
sync when you change a mechanism.

## Repository map

```
doc.go                v1 package docs
cache.go              v1 implementation
cache_test.go         v1 tests
example_test.go       v1 runnable examples (shown on pkg.go.dev)
get_bench_test.go     Get benchmarks comparing v1 with the v2 approach
v2/go.mod             the v2 module (separate from the root module)
v2/doc.go             v2 package docs, including benchmark tables
v2/cache.go           v2 Cache, options, shards, sweeping, eviction
v2/clock.go           Clock interface, CoarseClock, the shared default clock
v2/cache_test.go      v2 tests
v2/example_test.go    v2 runnable examples
v2/bench_test.go      v2 benchmarks across shard and goroutine counts
docs/design.md        internals of both versions
docs/migrating-from-anvil.md
.github/workflows/ci.yml
```

## Commands

The two directories are separate modules. `./...` at the root does **not**
include `v2/`, so run every command in both:

```sh
gofmt -l .                                  # must print nothing (covers both)
go vet ./...         && (cd v2 && go vet ./...)
go test -race ./...  && (cd v2 && go test -race ./...)
```

A single test or benchmark:

```sh
go test -run TestCacheGetOrSet -v .
(cd v2 && go test -run '^$' -bench BenchmarkCache -cpu 1,4 .)
go test -run '^$' -bench GetCompare -benchmem -count 5 -cpu 4 .
```

CI (`.github/workflows/ci.yml`) runs gofmt, vet, `test -race` on Go 1.27 and
stable, and staticcheck, for each module. Don't add a `go.work` file to the
repository (it is ignored by `.gitignore`); create one locally if you want
both modules in one workspace.

## Rules

- **Standard library only.** Neither module has dependencies; keep it that
  way. v1 and v2 must not import each other.
- **v1 is stable.** No breaking changes to its exported API or behavior.
  New features go to v2 first.
- **v2 is unstable** until v2.0.0 is released. Breaking changes are allowed
  but must be called out in the commit message and the package docs.
- **Keep the shared API in step.** v1 and v2 have the same core methods
  (`New`, `Get`, `Set`, `SetWithTTL`, `GetOrSet`, `Invalidate`, `Cleanup`,
  `RunCleanup`) with the same semantics, apart from expiry precision. A fix
  to the semantics of one usually belongs in both.
- **Performance claims need numbers.** Doc comments quote benchmark results;
  if you change a hot path, rerun the relevant benchmarks and update the
  tables (with machine, Go version and `-count`).
- **Tests use fake time.** v1 replaces `Cache.now`; v2 uses a fake `Clock`
  via `WithClock`. Don't add tests that sleep to wait for expiry.

## Documentation

- Every exported identifier has a doc comment that starts with its name and
  says what it does, its edge cases (zero/negative arguments, nil, panics)
  and its concurrency guarantees.
- Unexported code gets a comment when its purpose or an invariant isn't
  obvious: what lock the caller must hold, why a constant has its value.
- Package docs live in `doc.go`. Runnable examples go in `example_test.go`
  and must have an `// Output:` block.
- Update `README.md`, `docs/design.md` and the package docs in the same
  commit as the change they describe.
- Write in plain American English: short sentences, active voice, no
  marketing words.

## Commits

- Small, atomic commits on `master`: one logical change each, and every
  commit builds and passes tests in both modules.
- Subject: `<scope>: <imperative summary>`, lowercase after the colon, no
  period, at most about 72 characters. Scopes: `cache` (v1), `v2`, `docs`,
  `ci`. Repository-wide changes may omit the scope.
- Body: what changed and why, wrapped at 72 columns.

## Releases

- v1: tag `v1.x.y` on the root module.
- v2: tag `v2.x.y` (no directory prefix; the `v2` directory is the major
  version subdirectory). Until stable, tag pre-releases: `v2.0.0-alpha.N`,
  then `-beta.N`.
- Don't create or push tags unless asked.
