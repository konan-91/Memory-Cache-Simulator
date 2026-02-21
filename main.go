package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// CacheConfig Struct for holding config data for each???
type CacheConfig struct {
	Name              string `json:"name"`
	Size              int    `json:"size"`
	LineSize          int    `json:"line_size"`
	Kind              string `json:"kind"`
	ReplacementPolicy string `json:"replacement_policy"`
}

// Config Parent struct for holding Cache objects to match format in JSON
type Config struct {
	Caches []CacheConfig `json:"caches"`
}

// 1. Read the config JSON → know what caches to build
// 2. Read the trace file line by line → get a sequence of memory accesses
// 3. For each access, run it through the cache hierarchy
// 4. Print the stats as JSON

func main() {
	// Read the config JSON (for now, direct.json)
	jsonFile, err := os.Open("sample-inputs/direct.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Opened direct.json")
	defer jsonFile.Close() // Error check later?

	// Read in / decode JSON?
	var config Config
	decoder := json.NewDecoder(jsonFile)
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Test works
	fmt.Println(config)
}
