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

	scanConfig := &depextify.Config{
		NoBuiltins:   cfg.IgnoreBuiltins,
		NoCoreutils:  cfg.IgnoreCoreutils,
		NoCommon:     cfg.IgnoreCommon,
		ShowHidden:   cfg.ShowHidden,
		ExtraIgnores: cfg.Ignores,
		Excludes:     cfg.Excludes,
		ShowCount:    cfg.ShowCount,
		ShowPos:      cfg.ShowPos,
		UseColor:     cfg.UseColor,
		LexerName:    cfg.Lexer,
		StyleName:    cfg.Style,
		Format:       cfg.Format,
	}

	results, err := scanConfig.Scan(cfg.Target)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if cfg.Format == "json" {
		out, err := results.JSON(scanConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
		return
	}

	if cfg.Format == "yaml" {
		out, err := results.YAML(scanConfig)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error formatting YAML: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(out)
		return
	}

	fmt.Print(results.Format(scanConfig))
}