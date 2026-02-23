package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// CacheConfig struct to hold cache configuration parameters from JSON
type CacheConfig struct {
	Name              string `json:"name"`
	Size              int    `json:"size"`
	LineSize          int    `json:"line_size"`
	Kind              string `json:"kind"`
	ReplacementPolicy string `json:"replacement_policy"`
}

// Config Parent struct for holding cache configs
type Config struct {
	Caches []CacheConfig `json:"caches"`
}

// Cache simulates one level of the cache hierarchy.
type Cache struct {
	sets     [][]CacheLine
	numSets  int
	numWays  int
	lineSize int
	policy   string
	rrPtr    []int // one round-robin pointer per set
	tick     int
	hits     int
	misses   int
}

// CacheLine represents one line in the cache
type CacheLine struct {
	valid bool
	tag   uint64
	freq  int // for LFU: access frequency; for LRU: last-used tick; for RR: unused ???
	order int // for LRU: last-used tick; for RR: insertion index (round-robin pointer lives in the set) ???
}

// newCache constructor for Cache object
func newCache(cfg CacheConfig) *Cache {
	totalLines := cfg.Size / cfg.LineSize

	var numWays int
	switch cfg.Kind {
	case "direct":
		numWays = 1
	case "full":
		numWays = totalLines // fully associative so one big set
	case "2way":
		numWays = 2
	case "4way":
		numWays = 4
	case "8way":
		numWays = 8
	default:
		fmt.Fprintf(os.Stderr, "unknown cache kind: %s\n", cfg.Kind)
		os.Exit(1)
	}

	numSets := totalLines / numWays

	// Default replacement policy is roundrobin if not specified
	policy := cfg.ReplacementPolicy
	if policy == "" {
		policy = "rr"
	}

	// Allocate sets × ways grid of cache lines.
	sets := make([][]CacheLine, numSets)
	for i := range sets {
		sets[i] = make([]CacheLine, numWays)
	}

	return &Cache{
		sets:     sets,
		numSets:  numSets,
		numWays:  numWays,
		lineSize: cfg.LineSize,
		policy:   policy,
		rrPtr:    make([]int, numSets), // all start at 0
	}
}

// 1. Read the config JSON → know what caches to build
// 2. Read the trace file line by line → get a sequence of memory accesses
// 3. For each access, run it through the cache hierarchy
// 4. Print the stats as JSON

// Cache == entire structure at one level of hierarchy, Set == row in cache, CacheLine == container in set
func main() {
	// Must accept 2 args, path to JSON config and path to trace file
	if len(os.Args) != 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <config.json> <trace.txt>\n", os.Args[0])
		os.Exit(1)
	}
	configPath := os.Args[1]
	tracePath := os.Args[2]

	// Read the config JSON
	jsonFile, err := os.Open(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening config: %v\n", err)
		os.Exit(1)
	}
	defer jsonFile.Close()

	// Parse JSON and populate config struct
	var config Config
	if err := json.NewDecoder(jsonFile).Decode(&config); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing config: %v\n", err)
		os.Exit(1)
	}

	// Build the cache hierarhcy based on the config
	caches := make([]*Cache, len(config.Caches))
	for i, cfg := range config.Caches {
		caches[i] = newCache(cfg)
		fmt.Fprintf(os.Stderr, "built cache %q: %d sets × %d ways, line=%dB, policy=%s\n",
			cfg.Name, caches[i].numSets, caches[i].numWays, caches[i].lineSize, caches[i].policy) // Comment out later...
	}

	// Simulate the
	mainMem := 0
	processTrace(tracePath, caches, &mainMem)
}
