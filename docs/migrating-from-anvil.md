# Migrating from anvil

The caches used to be packages of `go.xchunk.org/anvil/v2`. They moved to
this repository unchanged; only the import paths are new:

| Old import path | New import path |
|---|---|
| `go.xchunk.org/anvil/v2/cache` | `go.xchunk.org/cache` |
| `go.xchunk.org/anvil/v2/cache/v2` | `go.xchunk.org/cache/v2` |

The package name is still `cache`, and the API and behavior are the same as
in anvil commit `a136340`, so no other code changes are needed. The copies in
anvil are deprecated and will not get new features.

## Steps

1. Rewrite the imports. From your module's root:

   ```sh
   grep -rl 'go.xchunk.org/anvil/v2/cache' --include='*.go' . | xargs sed -i \
     -e 's#"go.xchunk.org/anvil/v2/cache/v2"#"go.xchunk.org/cache/v2"#' \
     -e 's#"go.xchunk.org/anvil/v2/cache"#"go.xchunk.org/cache"#'
   ```

   On macOS, use `sed -i ''` instead of `sed -i`.

2. Add the new modules and drop anvil if nothing else uses it:

   ```sh
   go get go.xchunk.org/cache@latest      # if you used anvil's cache
   go get go.xchunk.org/cache/v2@latest   # if you used anvil's cache/v2
   go mod tidy
   ```

   v2 has only pre-release versions so far; `@latest` picks the newest of
   them.

3. Build and run your tests.

## Things to watch for

- **Mixed imports.** `*cache.Cache` from anvil and from this module are
  different types. If two packages of your program share a cache value,
  migrate them together, or the code won't compile.
- **Stability of v2.** As in anvil, v2 is unstable: its API may change
  between pre-releases. Pin a version in `go.mod` and read the release notes
  before upgrading. v1 (`go.xchunk.org/cache`) is stable.

## Coming from anvil v1

If you still use `anvil.NewCache` from `go.xchunk.org/anvil` (v1), first
apply anvil's own v1-to-v2 notes in its README: `anvil.NewCache` and
`anvil.CacheItem` became `cache.New` and `cache.Item`, and a ttl of zero or
less now means "never expires" instead of "expires immediately". Then follow
the steps above.
