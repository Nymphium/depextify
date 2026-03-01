// Package main is the entry point for the depextify CLI tool.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/nymphium/depextify/depextify"
)

func main() {
	cfg, err := parseFlags(os.Args[1:])
	if err != nil {
		if err.Error() != "no target specified" {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	if cfg.List != "" {
		targets := strings.Split(cfg.List, ",")
		all := false
		for _, t := range targets {
			if strings.TrimSpace(t) == "all" {
				all = true
				break
			}
		}

		if all {
			targets = []string{"builtins", "coreutils", "common"}
		}

		for _, t := range targets {
			t = strings.TrimSpace(t)
			switch t {
			case "builtins":
				printCategory(os.Stdout, "builtins", depextify.GetBuiltins(), cfg.UseColor)
			case "coreutils":
				printCategory(os.Stdout, "coreutils", depextify.GetCoreutils(), cfg.UseColor)
			case "common":
				printCategory(os.Stdout, "common", depextify.GetCommon(), cfg.UseColor)
			default:
				fmt.Fprintf(os.Stderr, "Warning: unknown category %q\n", t)
			}
		}
		return
	}

	// Extra ignores are already merged into cfg.Ignores in parseFlags

	scanConfig := cfg.ToConfig()

	results, err := scanConfig.Scan(cfg.Target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// We need to determine if target was a directory to pass to format config.
	isDir := false
	if info, err := os.Stat(cfg.Target); err == nil {
		isDir = info.IsDir()
	}

	formatConfig := cfg.ToFormatConfig(isDir)

	var formatter depextify.Formatter
	switch cfg.Format {
	case "json":
		formatter = &depextify.JSONFormatter{Config: formatConfig}
	case "yaml":
		formatter = &depextify.YAMLFormatter{Config: formatConfig}
	default:
		formatter = &depextify.TextFormatter{Config: formatConfig}
	}

	out, err := formatter.Format(results)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error formatting output: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(out)
}