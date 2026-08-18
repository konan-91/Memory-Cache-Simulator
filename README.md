# Memory Cache Simulator

A trace-driven CPU cache simulator written in Go. It replays a memory access
trace through a configurable cache hierarchy and reports hits, misses, and main
memory accesses per level.

Used to measure how cache size, line size, associativity, replacement policy,
and hierarchy depth affect hit rates on real workloads (SPEC CPU benchmark
traces: gcc, mcf, x264, xz, leela, omnetpp, perlbench, deepsjeng).

## What it models

- **Hierarchy**: any number of levels (L1, L2, L3, ...). An access walks down
  the levels until it hits; a miss at the last level counts as a main memory
  access.
- **Mapping**: direct-mapped, 2/4/8-way set associative, fully associative.
- **Replacement**: round-robin (default), LRU, LFU.
- **Unaligned and multi-line accesses**: an access is split into one lookup per
  L1-sized cache line it touches, so a request straddling a line boundary
  generates two lookups.

Misses allocate into the level that missed. The simulator tracks access counts
only. It does not model write-back, dirty bits, coherence, or timing.

## Build

Requires Go 1.25+.

```
go build -o cache_sim main.go
```

## Usage

```
./cache_sim <config.json> <trace file>
```

Results are printed to stdout as JSON.

### Config

Caches are listed in order, closest to the CPU first. `size` and `line_size`
are in bytes. `kind` is one of `direct`, `2way`, `4way`, `8way`, `full`.
`replacement_policy` is one of `rr`, `lru`, `lfu`, and defaults to `rr`.

```json
{
    "caches": [
        { "name": "L1", "size": 32768,    "line_size": 32, "kind": "direct" },
        { "name": "L2", "size": 131072,   "line_size": 64, "kind": "2way", "replacement_policy": "lfu" },
        { "name": "L3", "size": 16777216, "line_size": 64, "kind": "8way", "replacement_policy": "lru" }
    ]
}
```

### Trace format

Fixed-width text, one access per line: a 16-digit hex address at column 17 and
a 3-digit decimal access size in bytes at column 36. Lines shorter than 39
characters are skipped.

```
<16 hex chars> <16-hex address> <R/W> <3-digit size>
```

Trace files are not included in this repo, as they run to several GB each. Drop
your own into `trace-files/`.

### Output

```json
{
  "caches": [
    { "hits": 20066838, "misses": 334310, "name": "L1" },
    { "hits": 186774,   "misses": 147536, "name": "L2" },
    { "hits": 33048,    "misses": 114488, "name": "L3" }
  ],
  "main_memory_accesses": 114488
}
```

## Repository layout

```
main.go         simulator
sample-inputs/  cache configs covering each kind, policy, and hierarchy depth
sample-outputs/ results from running those configs against SPEC CPU traces
trace-files/    where to put your traces
```

## Implementation notes

Each cache is a `sets x ways` grid of lines. A lookup derives the set index and
tag from the address, scans the ways in the set for a matching valid tag, and on
a miss fills an invalid way if one exists or evicts by policy. LRU uses a
per-cache monotonic tick stored on each line; LFU uses a per-line access count;
round-robin keeps one pointer per set.

Traces are read through a 1 MB buffered reader and parsed by fixed byte offsets
rather than field splitting, since the input runs to hundreds of millions of
lines.
