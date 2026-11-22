// Package main is the entry point for the depextify CLI tool.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/nymphium/depextify/depextify"
	"gopkg.in/yaml.v3"
)

var errNoTarget = errors.New("no target specified")

// CLIConfig is the configuration merged from .depextify.yaml and the command-line flags.
type CLIConfig struct {
	depextify.Config       `yaml:",inline"`
	depextify.FormatConfig `yaml:",inline"`

	List       string `yaml:"-"`
	IgnoresStr string `yaml:"-"`
	Target     string `yaml:"-"`
	Format     string `yaml:"format"`
}

// boolFlag is a boolean flag that writes its value to p, negated when neg is set.
// A -x / -no-x pair writes the same field, so the last one given wins.
type boolFlag struct {
	p   *bool
	neg bool
}

func (b boolFlag) String() string {
	if b.p == nil {
		return "false"
	}
	return strconv.FormatBool(*b.p != b.neg)
}

func (b boolFlag) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return errors.New("parse error") // same wording as flag.Bool
	}
	*b.p = v != b.neg
	return nil
}

func (boolFlag) IsBoolFlag() bool { return true }

func isTTY() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// loadConfigFile reads .depextify.yaml from the current directory, or else from the home directory.
func loadConfigFile(cfg *CLIConfig) {
	paths := []string{".depextify.yaml"}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".depextify.yaml"))
	}
	for _, path := range paths {
		if content, err := os.ReadFile(path); err == nil {
			_ = yaml.Unmarshal(content, cfg)
			return
		}
	}
}

func setupFlagSet(cfg *CLIConfig) *flag.FlagSet {
	fs := flag.NewFlagSet("depextify", flag.ContinueOnError)

	fs.BoolVar(&cfg.ShowCount, "count", cfg.ShowCount, "show appearance count for each command")
	fs.BoolVar(&cfg.ShowPos, "pos", cfg.ShowPos, "show file position and full line for each command")
	fs.BoolVar(&cfg.ShowHidden, "hidden", cfg.ShowHidden, "scan hidden files and directories")
	fs.BoolVar(&cfg.Aggregate, "aggregate", cfg.Aggregate, "aggregate results across all files")

	// pair defines -name and -no-name on p; -name stores the negated value when neg is set.
	// The description is placed on -no-name.
	pair := func(name string, p *bool, neg bool, usage string) {
		fs.Var(boolFlag{p: p, neg: neg}, name, "")
		fs.Var(boolFlag{p: p, neg: !neg}, "no-"+name, usage)
	}
	pair("builtin", &cfg.NoBuiltins, true, "ignore/include shell built-in commands (default: true (ignore))")
	pair("coreutils", &cfg.NoCoreutils, true, "ignore/include coreutils commands (default: true (ignore))")
	pair("common", &cfg.NoCommon, true, "ignore/include common commands (grep, find, etc.) (default: true (ignore))")
	pair("color", &cfg.UseColor, false, "enable/disable colored output (default: auto)")
	pair("gitignore-aware", &cfg.GitignoreAware, false, "enable/disable gitignore awareness (default: true (enabled))")

	fs.StringVar(&cfg.List, "list", "", "comma-separated list of categories to list (builtins, coreutils, common) or \"all\"")
	fs.StringVar(&cfg.LexerName, "lexer", cfg.LexerName, "chroma lexer name")
	fs.StringVar(&cfg.StyleName, "style", cfg.StyleName, "chroma style name (env: DEPEXTIFY_STYLE)")
	fs.StringVar(&cfg.IgnoresStr, "ignores", "", "comma-separated list of commands to ignore")
	appendTo := func(p *[]string) func(string) error {
		return func(s string) error {
			*p = append(*p, strings.Split(s, ",")...)
			return nil
		}
	}
	fs.Func("query", "comma-separated list of commands to find; only they are reported, even if ignored", appendTo(&cfg.Query))
	fs.StringVar(&cfg.Format, "format", cfg.Format, "output format (text, json, yaml)")

	fs.Usage = func() {
		u := func(name string) string { return fs.Lookup(name).Usage }
		fmt.Fprintf(os.Stderr, "Usage: depextify [options] <file|directory>\n\nOptions:\n")
		fmt.Fprintf(os.Stderr, "  -count\n    \t%s\n", u("count"))
		fmt.Fprintf(os.Stderr, "  -pos\n    \t%s\n", u("pos"))
		fmt.Fprintf(os.Stderr, "  -hidden\n    \t%s\n", u("hidden"))
		fmt.Fprintf(os.Stderr, "  -aggregate\n    \t%s\n", u("aggregate"))
		fmt.Fprintf(os.Stderr, "  -[no-]builtin\n    \t%s\n", u("no-builtin"))
		fmt.Fprintf(os.Stderr, "  -[no-]coreutils\n    \t%s\n", u("no-coreutils"))
		fmt.Fprintf(os.Stderr, "  -[no-]common\n    \t%s\n", u("no-common"))
		fmt.Fprintf(os.Stderr, "  -[no-]color\n    \t%s\n", u("no-color"))
		fmt.Fprintf(os.Stderr, "  -[no-]gitignore-aware\n    \t%s\n", u("no-gitignore-aware"))
		fmt.Fprintf(os.Stderr, "  -ignores string\n    \t%s\n", u("ignores"))
		fmt.Fprintf(os.Stderr, "  -query string\n    \t%s\n", u("query"))
		fmt.Fprintf(os.Stderr, "  -list string\n    \t%s\n", u("list"))
		fmt.Fprintf(os.Stderr, "  -lexer string\n    \t%s (default: %q)\n", u("lexer"), depextify.DefaultLexer)
		fmt.Fprintf(os.Stderr, "  -style string\n    \t%s (default: %q)\n", u("style"), depextify.DefaultStyle)
		fmt.Fprintf(os.Stderr, "  -format string\n    \t%s (default: \"text\")\n", u("format"))
	}
	return fs
}

func parseFlags(args []string) (*CLIConfig, error) {
	cfg := &CLIConfig{
		Config: depextify.Config{
			NoBuiltins:     true,
			NoCoreutils:    true,
			NoCommon:       true,
			GitignoreAware: true,
		},
		FormatConfig: depextify.FormatConfig{
			UseColor:  isTTY(),
			LexerName: depextify.DefaultLexer,
			StyleName: depextify.DefaultStyle,
		},
		Format: "text",
	}

	loadConfigFile(cfg)

	fs := setupFlagSet(cfg)

	// flag stops at the first positional argument; resume after it so that flags may follow the target.
	var positional []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			break
		}
		positional = append(positional, fs.Arg(0))
		args = fs.Args()[1:]
	}

	if envStyle := os.Getenv("DEPEXTIFY_STYLE"); envStyle != "" {
		cfg.StyleName = envStyle
	}

	if cfg.List != "" {
		if len(positional) > 0 || cfg.ShowCount || cfg.ShowPos || cfg.IgnoresStr != "" || cfg.ShowHidden || (cfg.Format != "text" && cfg.Format != "") {
			return nil, fmt.Errorf("-list flag cannot be used with other arguments or flags")
		}
		return cfg, nil
	}

	if len(positional) < 1 {
		fs.Usage()
		return nil, errNoTarget
	}

	cfg.Target = positional[0]
	if cfg.IgnoresStr != "" {
		cfg.ExtraIgnores = append(cfg.ExtraIgnores, strings.Split(cfg.IgnoresStr, ",")...)
	}

	return cfg, nil
}

func printCategory(w io.Writer, name string, commands []string, useColor bool) {
	header := name + ":"
	if useColor {
		header = "\033[36m" + name + "\033[0m" + ":"
	}
	_, _ = fmt.Fprintln(w, header)
	for chunk := range slices.Chunk(commands, 5) {
		_, _ = fmt.Fprintf(w, "  %s\n", strings.Join(chunk, ", "))
	}
	_, _ = fmt.Fprintln(w)
}
