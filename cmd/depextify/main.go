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

	if cfg.list != "" {
		targets := strings.Split(cfg.list, ",")
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
				printCategory("builtins", depextify.GetBuiltins(), cfg.useColor)
			case "coreutils":
				printCategory("coreutils", depextify.GetCoreutils(), cfg.useColor)
			case "common":
				printCategory("common", depextify.GetCommon(), cfg.useColor)
			default:
				fmt.Fprintf(os.Stderr, "Warning: unknown category %q\n", t)
			}
		}
		return
	}

	info, err := os.Stat(cfg.target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	extraIgnores := []string{}
	if cfg.ignores != "" {
		extraIgnores = strings.Split(cfg.ignores, ",")
	}

	results, err := depextify.Scan(cfg.target, cfg.ignoreBuiltins, cfg.ignoreCoreutils, cfg.ignoreCommon, cfg.showHidden, extraIgnores)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(results.Format(cfg.showCount, cfg.showPos, info.IsDir(), cfg.useColor, cfg.lexer, cfg.style))
}