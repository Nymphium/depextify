package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nymphium/depextify/depextify"
)

type config struct {
	showCount       bool
	showPos         bool
	ignoreCoreutils bool
	ignoreCommon    bool
	useColor        bool
	list            string
	lexer           string
	style           string
	ignores         string
	target          string
}

const (
	colorReset = "\033[0m"
	colorCyan  = "\033[36m"
)

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func parseFlags() (*config, error) {
	cfg := &config{
		ignoreCoreutils: true,
		ignoreCommon:    true,
		useColor:        isTTY(),
	}
	fs := flag.NewFlagSet("depextify", flag.ContinueOnError)

	fs.BoolVar(&cfg.showCount, "count", false, "show appearance count for each command")
	fs.BoolVar(&cfg.showPos, "pos", false, "show file position and full line for each command")

	// Pointers for [no-] flags. Descriptions are placed in one of the pair.
	pCore := fs.Bool("coreutils", false, "")
	pNoCore := fs.Bool("no-coreutils", true, "ignore/include coreutils commands (default: true (ignore))")
	pCommon := fs.Bool("common", false, "")
	pNoCommon := fs.Bool("no-common", true, "ignore/include common commands (grep, find, etc.) (default: true (ignore))")
	pColor := fs.Bool("color", true, "")
	pNoColor := fs.Bool("no-color", false, "enable/disable colored output (default: auto)")

	fs.StringVar(&cfg.list, "list", "", "comma-separated list of categories to list (builtins, coreutils, common) or \"all\"")
	fs.StringVar(&cfg.lexer, "lexer", "bash", "chroma lexer name")
	fs.StringVar(&cfg.style, "style", "monokai", "chroma style name (env: DEPEXTIFY_STYLE)")
	fs.StringVar(&cfg.ignores, "ignores", "", "comma-separated list of commands to ignore")

	fs.Usage = func() {
		u := func(name string) string { return fs.Lookup(name).Usage }
		fmt.Fprintf(os.Stderr, "Usage: depextify [options] <file|directory>\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -count\n    \t%s\n", u("count"))
		fmt.Fprintf(os.Stderr, "  -pos\n    \t%s\n", u("pos"))
		fmt.Fprintf(os.Stderr, "  -[no-]coreutils\n    \t%s\n", u("no-coreutils"))
		fmt.Fprintf(os.Stderr, "  -[no-]common\n    \t%s\n", u("no-common"))
		fmt.Fprintf(os.Stderr, "  -[no-]color\n    \t%s\n", u("no-color"))
		fmt.Fprintf(os.Stderr, "  -ignores string\n    \t%s\n", u("ignores"))
		fmt.Fprintf(os.Stderr, "  -list string\n    \t%s\n", u("list"))
		fmt.Fprintf(os.Stderr, "  -lexer string\n    \t%s (default: \"bash\")\n", u("lexer"))
		fmt.Fprintf(os.Stderr, "  -style string\n    \t%s (default: \"monokai\")\n", u("style"))
	}

	var args []string
	var flags []string
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if strings.HasPrefix(arg, "-") {
			flags = append(flags, arg)
		} else {
			args = append(args, arg)
		}
	}

	if err := fs.Parse(flags); err != nil {
		return nil, err
	}

	// Honor the order of flags provided on CLI (last one wins)
	for _, f := range flags {
		name := strings.TrimLeft(f, "-")
		if idx := strings.Index(name, "="); idx != -1 {
			name = name[:idx]
		}
		switch name {
		case "coreutils":
			cfg.ignoreCoreutils = !*pCore
		case "no-coreutils":
			cfg.ignoreCoreutils = *pNoCore
		case "common":
			cfg.ignoreCommon = !*pCommon
		case "no-common":
			cfg.ignoreCommon = *pNoCommon
		case "color":
			cfg.useColor = *pColor
		case "no-color":
			cfg.useColor = !*pNoColor
		}
	}

	if envStyle := os.Getenv("DEPEXTIFY_STYLE"); envStyle != "" {
		cfg.style = envStyle
	}

	if cfg.list != "" {
		if len(args) > 0 || cfg.showCount || cfg.showPos || cfg.ignores != "" {
			return nil, fmt.Errorf("-list flag cannot be used with other arguments or flags")
		}
		return cfg, nil
	}

	if len(args) < 1 {
		fs.Usage()
		return nil, fmt.Errorf("no target specified")
	}

	cfg.target = args[0]
	return cfg, nil
}

func printCategory(name string, commands []string, useColor bool) {
	header := name + ":"
	if useColor {
		header = colorCyan + name + colorReset + ":"
	}
	fmt.Println(header)
	for i := 0; i < len(commands); i += 5 {
		end := i + 5
		if end > len(commands) {
			end = len(commands)
		}
		fmt.Printf("  %s\n", strings.Join(commands[i:end], ", "))
	}
	fmt.Println()
}

func main() {
	cfg, err := parseFlags()
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

	results, err := depextify.Scan(cfg.target, cfg.ignoreCoreutils, cfg.ignoreCommon, extraIgnores)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(results.Format(cfg.showCount, cfg.showPos, info.IsDir(), cfg.useColor, cfg.lexer, cfg.style))
}