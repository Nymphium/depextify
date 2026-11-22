# depextify

`depextify` is a tool to collect and display command dependencies from shell scripts and other configuration files. It helps you identify what external binaries your project depends on.

## Features

- **Polyglot Analysis**: Extracts dependencies from:
  - Shell scripts (`.sh`, `.bash`, `.zsh`, etc., or files with shebangs)
  - `Makefile`
  - `Dockerfile` (commands in `RUN` instructions)
  - GitHub Actions Workflows (`.github/workflows/*.yml`)
  - `Taskfile.yml`
- **Smart Filtering**: Built-in lists for shell built-ins, GNU coreutils, and common tools to help you focus on actual external dependencies.
- **Hierarchical Ignore Support**: Respects `.gitignore` and `.depextifyignore` files in each directory during traversal.
- **Detailed Reporting**: Show occurrences, line numbers, and even the full line where each command is used.
- **Result Aggregation**: Summarize findings across the entire project with a single command.
- **Syntax Highlighting**: Beautifully highlighted output using [chroma](https://github.com/alecthomas/chroma).
- **Multiple Formats**: Export results as Text, JSON, YAML, or Markdown.
- **Command Policies**: Fail the run when denied commands are used, or when commands outside an allow list are reported.
- **Configurable**: Project-specific settings via `.depextify.yaml`.

## Installation

### Go

```sh
go install github.com/nymphium/depextify/cmd/depextify@latest
```

### Nix

You can run `depextify` directly without installing it using Nix:

```sh
nix run github:nymphium/depextify -- examples/sample.sh
```

## Usage

```sh
depextify [options] <file|directory>
```

By default, it recursively scans directories and filters out shell built-ins, GNU coreutils, and common tools (like `grep`, `sed`, `awk`) to show meaningful external dependencies.

A command run through a variable is reported by the value assigned to that variable in the same file: `CC=gcc` followed by `$CC main.c` reports `gcc` (likewise `CC = gcc` and `$(CC)` in a Makefile, `ARG`/`ENV` in a Dockerfile, and `env:` in YAML). Commands whose names are not known statically, such as `$1`, a variable set by the environment, or `"$DIR/run.sh"`, are not reported, and neither are assignments to variables that are never run as commands. Quotes and escapes are removed from command names, so `"jq"`, `'jq'` and `\jq` are all reported as `jq`.

### Options

- `-count`: Show the number of occurrences for each command.
- `-pos`: Show the file position (line number) and the full line where each command is used.
- `-hidden`: Scan hidden files and directories (default: ignore). `.git` is never scanned.
- `-aggregate`: Aggregate results across all scanned files instead of reporting per file.
- `-hints`: Print notes on stderr about commands whose versions or variants differ between systems, such as `python` (Python 2 or 3) or `sed` (GNU or BSD). Hidden commands are checked too, since most of these are coreutils or common tools.
- `-[no-]builtin`: Ignore/include shell built-in commands (default: ignore).
- `-[no-]coreutils`: Ignore/include GNU coreutils in the output (default: ignore).
- `-[no-]common`: Ignore/include common tools (grep, sed, awk, etc.) in the output (default: ignore).
- `-[no-]color`: Enable/disable colored output (default: auto).
- `-[no-]gitignore-aware`: Enable/disable respect for `.gitignore` files (default: enabled).
- `-ignores=cmd1,cmd2,...`: Comma-separated list of additional commands to ignore.
- `-interactive`, `-i`: Ask, for each reported command, whether to keep or ignore it, and add the ignored ones to `ignores` in the configuration file that was loaded (or `./.depextify.yaml` when there is none). Other keys and comments in the file are kept.
- `-query=cmd1,cmd2,...`: Report only these commands, to find the files (and, with `-pos`, the lines) that use them. The ignore rules do not apply to queried commands.
- `-deny=cmd1,cmd2,...`: Fail (exit status 1) when any of these commands is used, even if an ignore rule hides it, printing each use as `file:line` on stderr.
- `-allow=cmd1,cmd2,...`: Fail (exit status 1) when a reported command is not in this list, printing each use as `file:line` on stderr.
- `-generate=<brew|apt|nix>`: Print an installation template for the reported commands instead of the result: a Brewfile, an `apt-get install` command, or a Nix `mkShell` expression. Package names are assumed to equal command names, so check them; paths are left out.
- `-lookup` (experimental): Print the packages that provide each reported command instead of the result, using the first installed tool of `nix-locate` (needs a `nix-index` database), `apt-file`, `pacman` (needs `pacman -Fy`) and `dnf`.
- `-list=cat1,cat2,...`: List ignored commands in specified categories (`builtins`, `coreutils`, `common`) or `all`, then exit.
- `-lexer <name>`: Specify the [chroma](https://github.com/alecthomas/chroma) lexer for syntax highlighting (default: `bash`).
- `-style <name>`: Specify the chroma style for syntax highlighting (default: `monokai`). Can also be set via the `DEPEXTIFY_STYLE` environment variable.
- `-format <type>`: Specify output format (`text`, `json`, `yaml`, `markdown`). Default: `text`. `markdown` prints nested lists that can be pasted into a README as a dependency list.

## Configuration

`depextify` can be configured using a `.depextify.yaml` file. It searches for this file in the current directory, and if not found, in your home directory.

Example `.depextify.yaml`:

```yaml
no_builtins: true
no_coreutils: true
no_common: true
show_count: true
show_pos: true
use_color: true
gitignore_aware: true
aggregate: false
hints: false
lexer: bash
style: monokai
format: text
ignores:
  - my-custom-command
excludes:
  - vendor/
  - node_modules/
deny:
  - nc
  - telnet
allow: []
```

`-ignores`, `-allow` and `-deny` add to the lists in the configuration file.

## Ignoring Files

`depextify` respects `.gitignore` and `.depextifyignore` files. When scanning a directory, it looks for these files in the current directory and all subdirectories, scoping the ignore patterns accordingly. It uses the same syntax as `.gitignore`.

## Examples

### Scan a directory

```sh
$ depextify .
examples/sample.sh
  notify-send
  jq
```

### Show positions and counts

```sh
$ depextify -pos -count examples/sample.sh
jq: 1
  24:  echo "$RESPONSE" | jq '.status'
notify-send: 1
  28:  notify-send "Task Finished" "Backup check complete"
```

### Find the files that use a command

```sh
$ depextify -query=jq,curl -pos examples
examples/Makefile
  curl:
    13:  curl -X POST https://example.com/deploy
examples/sample.sh
  curl:
    25:  RESPONSE=$(curl -s https://api.example.invalid/status)
  jq:
    26:  echo "$RESPONSE" | jq '.status'
```

### Write a dependency list for a README

```sh
$ depextify -format=markdown -aggregate examples
- `examples`
  - `apk`
  - `go`
  - `jq`
  - `notify-send`
```

### Fail on denied commands

`sudo` is a common tool and hidden by default, but `-deny` still finds it.

```sh
$ depextify -deny=sudo,nc deploy.sh
jq
nc

deploy.sh:3: command "nc" is denied
deploy.sh:1: command "sudo" is denied
$ echo $?
1
```

### Generate an installation template

```sh
$ depextify -generate=nix examples
# Package names are assumed to equal command names; check them before use.
{ pkgs ? import <nixpkgs> { } }:
pkgs.mkShell {
  packages = [
    pkgs.apk
    pkgs.go
    pkgs.jq
    pkgs.notify-send
  ];
}
```

Here `notify-send` is provided by the `libnotify` package, so that line needs editing.

### Build the ignore list interactively

```sh
$ depextify -i examples
apk (1 use): [k]eep, [i]gnore, [q]uit? i
go (3 uses): [k]eep, [i]gnore, [q]uit? k
jq (1 use): [k]eep, [i]gnore, [q]uit? k
notify-send (1 use): [k]eep, [i]gnore, [q]uit? k
Added apk to the ignores in .depextify.yaml.
```

### Look up the packages that provide the commands

```sh
$ depextify -lookup examples/sample.sh
# Packages found with dnf
jq: jq
notify-send: libnotify
```

### Include coreutils and common tools

```sh
$ depextify -builtin -coreutils -common examples/sample.sh
curl
date
echo
find
jq
mkdir
notify-send
rm
tee
touch
wc
xargs
```

## License

[MIT](/LICENSE)
