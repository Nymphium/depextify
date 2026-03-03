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
- **Multiple Formats**: Export results as Text, JSON, or YAML.
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

### Options

- `-count`: Show the number of occurrences for each command.
- `-pos`: Show the file position (line number) and the full line where each command is used.
- `-hidden`: Scan hidden files and directories (default: ignore).
- `-aggregate`: Aggregate results across all scanned files instead of reporting per file.
- `-[no-]builtin`: Ignore/include shell built-in commands (default: ignore).
- `-[no-]coreutils`: Ignore/include GNU coreutils in the output (default: ignore).
- `-[no-]common`: Ignore/include common tools (grep, sed, awk, etc.) in the output (default: ignore).
- `-[no-]color`: Enable/disable colored output (default: auto).
- `-[no-]gitignore-aware`: Enable/disable respect for `.gitignore` files (default: enabled).
- `-ignores=cmd1,cmd2,...`: Comma-separated list of additional commands to ignore.
- `-list=cat1,cat2,...`: List ignored commands in specified categories (`builtins`, `coreutils`, `common`) or `all`, then exit.
- `-lexer <name>`: Specify the [chroma](https://github.com/alecthomas/chroma) lexer for syntax highlighting (default: `bash`).
- `-style <name>`: Specify the chroma style for syntax highlighting (default: `monokai`). Can also be set via the `DEPEXTIFY_STYLE` environment variable.
- `-format <type>`: Specify output format (`text`, `json`, `yaml`). Default: `text`.

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
lexer: bash
style: monokai
format: text
ignores:
  - my-custom-command
excludes:
  - vendor/
  - node_modules/
```

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
