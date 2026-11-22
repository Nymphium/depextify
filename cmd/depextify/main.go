// Package main is the entry point for the depextify CLI tool.
package main

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/nymphium/depextify/depextify"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if !errors.Is(err, errNoTarget) {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	if cfg.List != "" {
		printLists(cfg.List, cfg.UseColor)
		return
	}

	results, err := cfg.Scan(cfg.Target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	formatConfig := cfg.FormatConfig
	if info, err := os.Stat(cfg.Target); err == nil {
		formatConfig.IsDirectory = info.IsDir()
	}

	var formatter depextify.Formatter
	switch cfg.Format {
	case "json":
		formatter = &depextify.JSONFormatter{Config: &formatConfig}
	case "yaml":
		formatter = &depextify.YAMLFormatter{Config: &formatConfig}
	default:
		formatter = &depextify.TextFormatter{Config: &formatConfig}
	}

	out, err := formatter.Format(results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}

// printLists prints the comma-separated categories in list, or every category for "all".
func printLists(list string, useColor bool) {
	categories := map[string]func() []string{
		"builtins":  depextify.GetBuiltins,
		"coreutils": depextify.GetCoreutils,
		"common":    depextify.GetCommon,
	}

	targets := strings.Split(list, ",")
	if slices.ContainsFunc(targets, func(t string) bool { return strings.TrimSpace(t) == "all" }) {
		targets = []string{"builtins", "coreutils", "common"}
	}

	for _, t := range targets {
		t = strings.TrimSpace(t)
		get, ok := categories[t]
		if !ok {
			fmt.Fprintf(os.Stderr, "Warning: unknown category %q\n", t)
			continue
		}
		printCategory(os.Stdout, t, get(), useColor)
	}
}
