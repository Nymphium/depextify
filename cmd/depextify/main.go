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
				printCategory(os.Stdout, "builtins", depextify.GetBuiltins(), cfg.useColor)
			case "coreutils":
				printCategory(os.Stdout, "coreutils", depextify.GetCoreutils(), cfg.useColor)
			case "common":
				printCategory(os.Stdout, "common", depextify.GetCommon(), cfg.useColor)
			default:
				fmt.Fprintf(os.Stderr, "Warning: unknown category %q\n", t)
			}
		}
		return
	}

	extraIgnores := []string{}
	if cfg.ignores != "" {
		extraIgnores = strings.Split(cfg.ignores, ",")
	}

	scanConfig := &depextify.Config{
		NoBuiltins:   cfg.ignoreBuiltins,
		NoCoreutils:  cfg.ignoreCoreutils,
		NoCommon:     cfg.ignoreCommon,
		ShowHidden:   cfg.showHidden,
		ExtraIgnores: extraIgnores,
		ShowCount:    cfg.showCount,
		ShowPos:      cfg.showPos,
		UseColor:     cfg.useColor,
		LexerName:    cfg.lexer,
		StyleName:    cfg.style,
	}

	results, err := scanConfig.Scan(cfg.target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(results.Format(scanConfig))
}
