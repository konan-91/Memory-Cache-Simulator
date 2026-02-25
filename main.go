package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
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

// fillWay writes a new tag into the chosen way and initialises metadata
func (c *Cache) fillWay(set []CacheLine, way int, tag uint64, setIndex int) {
	set[way].valid = true
	set[way].tag = tag
	set[way].freq = 1
	set[way].order = c.tick
}

// chooseVictim selects which way to evict based on the replacement policy.
func (c *Cache) chooseVictim(set []CacheLine, setIndex int) int {
	switch c.policy {
	case "rr":
		// Get eviction target at round-robin pointer, then advance pointer for next time
		victim := c.rrPtr[setIndex]
		c.rrPtr[setIndex] = (c.rrPtr[setIndex] + 1) % c.numWays // Mod for wraparound
		return victim

	case "lru":
		// Evict the way with the lowest order timestamp
		victim := 0
		for w := 1; w < len(set); w++ {
			if set[w].order < set[victim].order {
				victim = w
			}
		}
		return victim

	case "lfu":
		// Evict the way with the lowest frequency count
		victim := 0
		for w := 1; w < len(set); w++ {
			// Smaller index wins ties (first seen)
			if set[w].freq < set[victim].freq {
				victim = w
			}
		}
		return victim

	default:
		return 0
	}
}

// access simulates one cache access for a given memory address. Returns boolean for hit/miss
func (c *Cache) access(addr uint64) bool {
	c.tick++ // Timestamp for LRU

	// Find which set it maps to, and the tag that identifies it within that set
	setIndex := (addr / uint64(c.lineSize)) % uint64(c.numSets)
	tag := addr / uint64(c.lineSize) / uint64(c.numSets)
	// Get set from cache
	set := c.sets[setIndex]

	// Check for hit by scanning all ways in the set for a matching tag
	for w := range set {
		if set[w].valid && set[w].tag == tag {
			// Hit, so update metadata for LRU/LFU & exit early
			set[w].order = c.tick
			set[w].freq++
			c.hits++
			return true
		}
	}

	// Else cache miss...
	c.misses++

	// Check for an empty way in set to avoid eviction
	for w := range set {
		if !set[w].valid {
			c.fillWay(set, w, tag, int(setIndex))
			return false
		}
	}

	// Else, evict according to replacement policy
	victim := c.chooseVictim(set, setIndex)
	c.fillWay(set, victim, tag, int(setIndex))
	return false
}

// accessHierarchy walks through cache levels until it finds a hit or exhausts the hierarchy
func accessHierarchy(caches []*Cache, lineAddr uint64, mainMem *int) {
	for i, cache := range caches {
		hit := cache.access(lineAddr)
		if hit {
			return
		}
		// If last cache, access goes to main memory
		if i == len(caches)-1 {
			*mainMem++
		}
	}
}

// simulate splits memory accesses into cacheline chunks and routes each through hierarchy
func simulate(caches []*Cache, addr uint64, size int, mainMem *int) {
	// L1 line size will be smallest line size, so use this value for splitting access
	l1LineSize := uint64(caches[0].lineSize)

	// Determine how many lines the access spans, in terms of l1LineSize
	firstLine := addr / l1LineSize
	lastLine := (addr + uint64(size) - 1) / l1LineSize

	// For each line touched by access, route through hierarchy
	for line := firstLine; line <= lastLine; line++ {
		lineAddr := line * l1LineSize
		accessHierarchy(caches, lineAddr, mainMem)
	}
}

// processTrace opens and reads trace file, parses each line, calls simulate()
func processTrace(path string, caches []*Cache, mainMem *int) {
	f, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening trace: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	// Read trace file line by line
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Split trace line on whitespace
		fields := strings.Fields(line)

		// Parse memory address
		addr, err := strconv.ParseUint(fields[1], 16, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad address %q: %v\n", fields[1], err)
			continue
		}

		// Parse access size
		size, err := strconv.Atoi(fields[3])
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad size %q: %v\n", fields[3], err)
			continue
		}

		simulate(caches, addr, size, mainMem)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "scanner error: %v\n", err)
		os.Exit(1)
	}
}

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

	// Simulate traces
	mainMem := 0
	processTrace(tracePath, caches, &mainMem)
}
