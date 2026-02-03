package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nymphium/depextify/depextify"
)

type config struct {
	showCount     bool
	showPos       bool
	noCoreutils   bool
	noCommon      bool
	noColor       bool
	listBuiltins  bool
	listCoreutils bool
	listCommon    bool
	lexer         string
	style         string
	ignores       string
	target        string
}

func parseFlags() (*config, error) {
	cfg := &config{}
	fs := flag.NewFlagSet("depextify", flag.ContinueOnError)
	fs.BoolVar(&cfg.showCount, "count", false, "show appearance count for each command")
	fs.BoolVar(&cfg.showPos, "pos", false, "show file position and full line for each command")
	fs.BoolVar(&cfg.noCoreutils, "no-coreutils", true, "ignore coreutils commands")
	fs.BoolVar(&cfg.noCommon, "no-common", true, "ignore common commands (grep, find, etc.)")
	fs.BoolVar(&cfg.noColor, "no-color", false, "disable colored output")
	fs.BoolVar(&cfg.listBuiltins, "list-builtins", false, "list shell built-in commands")
	fs.BoolVar(&cfg.listCoreutils, "list-coreutils", false, "list GNU Coreutils commands")
	fs.BoolVar(&cfg.listCommon, "list-common", false, "list common shell commands")
	fs.StringVar(&cfg.lexer, "lexer", "bash", "chroma lexer name")
	fs.StringVar(&cfg.style, "style", "monokai", "chroma style name (env: DEPEXTIFY_STYLE)")
	fs.StringVar(&cfg.ignores, "ignores", "", "comma-separated list of commands to ignore")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: depextify [options] <file|directory>\n")
		fs.PrintDefaults()
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

	// Environment variable override for style
	if envStyle := os.Getenv("DEPEXTIFY_STYLE"); envStyle != "" {
		cfg.style = envStyle
	}

	// Check if any list flag is set
	if cfg.listBuiltins || cfg.listCoreutils || cfg.listCommon {
		if len(args) > 0 || cfg.showCount || cfg.showPos || cfg.ignores != "" {
			return nil, fmt.Errorf("list flags cannot be used with other arguments or flags")
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

func main() {
	cfg, err := parseFlags()
	if err != nil {
		if err.Error() != "no target specified" {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		os.Exit(1)
	}

	if cfg.listBuiltins {
		fmt.Println(strings.Join(depextify.GetBuiltins(), "\n"))
		return
	}
	if cfg.listCoreutils {
		fmt.Println(strings.Join(depextify.GetCoreutils(), "\n"))
		return
	}
	if cfg.listCommon {
		fmt.Println(strings.Join(depextify.GetCommon(), "\n"))
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

	results, err := depextify.Scan(cfg.target, cfg.noCoreutils, cfg.noCommon, extraIgnores)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	useColor := false
	if !cfg.noColor {
		if fileInfo, err := os.Stdout.Stat(); err == nil {
			if (fileInfo.Mode() & os.ModeCharDevice) != 0 {
				useColor = true
			}
		}
	}

	fmt.Print(results.Format(cfg.showCount, cfg.showPos, info.IsDir(), useColor, cfg.lexer, cfg.style))
}
