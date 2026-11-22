// Package main is the entry point for the depextify CLI tool.
package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/nymphium/depextify/depextify"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// run executes the CLI with args and returns the exit status.
func run(args []string, stdout, stderr io.Writer) int {
	fail := func(format string, a ...any) int {
		_, _ = fmt.Fprintf(stderr, format, a...)
		return 1
	}

	cfg, err := parseFlags(args)
	if err != nil {
		if errors.Is(err, errNoTarget) {
			return 1
		}
		return fail("Error: %v\n", err)
	}

	if cfg.List != "" {
		printLists(stdout, stderr, cfg.List, cfg.UseColor)
		return 0
	}

	results, err := cfg.Scan(cfg.Target)
	if err != nil {
		return fail("Error: %v\n", err)
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
	case "markdown":
		formatter = &depextify.MarkdownFormatter{Config: &formatConfig}
	default:
		formatter = &depextify.TextFormatter{Config: &formatConfig}
	}

	if cfg.Generate != "" {
		_, _ = fmt.Fprint(stdout, generate(cfg.Generate, results.Commands()))
	} else {
		out, err := formatter.Format(results)
		if err != nil {
			return fail("Error formatting output: %v\n", err)
		}
		_, _ = fmt.Fprintln(stdout, out)
	}

	if !cfg.Hints && len(cfg.Allow) == 0 && len(cfg.Deny) == 0 {
		return 0
	}
	all, err := scanAll(cfg.Config, cfg.Target)
	if err != nil {
		return fail("Error: %v\n", err)
	}
	if cfg.Hints {
		for _, cmd := range all.Commands() {
			if hint := depextify.GetHint(cmd); hint != "" {
				_, _ = fmt.Fprintf(stderr, "hint: %s: %s\n", cmd, hint)
			}
		}
	}
	msgs := violations(cfg, all)
	for _, msg := range msgs {
		_, _ = fmt.Fprintln(stderr, msg)
	}
	if len(msgs) > 0 {
		return 1
	}
	return 0
}

// scanAll scans target without ignore rules, Query or aggregation, for checks that must see every command.
// ponytail: walks target a second time; derive both results from one walk if large trees make this slow.
func scanAll(c depextify.Config, target string) (depextify.ScanResult, error) {
	c.NoBuiltins, c.NoCoreutils, c.NoCommon = false, false, false
	c.ExtraIgnores, c.Query, c.Aggregate = nil, nil, false
	return c.Scan(target)
}

// violations describes each occurrence in all of a command that cfg.Deny lists,
// or, when cfg.Allow is set, of a command the ignore rules keep but cfg.Allow lacks.
func violations(cfg *CLIConfig, all depextify.ScanResult) []string {
	var msgs []string
	for _, path := range slices.Sorted(maps.Keys(all)) {
		for _, cmd := range slices.Sorted(maps.Keys(all[path])) {
			var reason string
			switch {
			case slices.Contains(cfg.Deny, cmd):
				reason = "is denied"
			case len(cfg.Allow) > 0 && !cfg.Ignored(cmd) && !slices.Contains(cfg.Allow, cmd):
				reason = "is not allowed"
			default:
				continue
			}
			for _, occ := range all[path][cmd] {
				msgs = append(msgs, fmt.Sprintf("%s:%d: command %q %s", path, occ.Line, cmd, reason))
			}
		}
	}
	return msgs
}

// printLists prints the comma-separated categories in list, or every category for "all".
func printLists(stdout, stderr io.Writer, list string, useColor bool) {
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
			_, _ = fmt.Fprintf(stderr, "Warning: unknown category %q\n", t)
			continue
		}
		printCategory(stdout, t, get(), useColor)
	}
}
