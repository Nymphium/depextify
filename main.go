package main

import (
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/nymphium/depextify/depextify"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: depextify <file>")
		os.Exit(1)
	}

	f, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	commands, err := depextify.Do(f)
	if err != nil {
		fmt.Fprintf(os.Stderr, "depextify error: %v\n", err)
		os.Exit(1)
	}

	keys := slices.Sorted(maps.Keys(commands))

	for _, cmd := range keys {
		fmt.Println(cmd)
	}
}
