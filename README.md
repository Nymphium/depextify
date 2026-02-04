# depextify

`depextify` is a tool to collect and display command dependencies from shell scripts.

## Installation

```sh
go install github.com/nymphium/depextify/cmd/depextify@latest
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
- `-[no-]builtin`: Ignore/include shell built-in commands (default: ignore).
- `-[no-]coreutils`: Ignore/include GNU coreutils in the output (default: ignore).
- `-[no-]common`: Ignore/include common tools (grep, sed, awk, etc.) in the output (default: ignore).
- `-[no-]color`: Enable/disable colored output (default: auto).
- `-ignores=cmd1,cmd2,...`: Comma-separated list of additional commands to ignore.
- `-list=cat1,cat2,...`: List ignored commands in specified categories (`builtins`, `coreutils`, `common`) or `all`, then exit.
- `-lexer <name>`: Specify the [chroma](https://github.com/alecthomas/chroma) lexer for syntax highlighting (default: `bash`).
- `-style <name>`: Specify the chroma style for syntax highlighting (default: `monokai`). Can also be set via the `DEPEXTIFY_STYLE` environment variable.

## Examples

### Scan a directory

```sh
$ depextify .
examples/test.sh
  notify-send
  tee
```

### Show positions and counts

```sh
$ depextify -pos -count examples/test.sh
notify-send: 1
  15:  notify-send "Test"
tee: 1
  12:  echo "Hello" | tee /tmp/test.log
```

### Include coreutils and common tools

```sh
$ depextify -builtin -coreutils -common examples/test.sh
curl
date
find
jq
mkdir
notify-send
tee
touch
wc
xargs
```

## License

[MIT](/LICENSE)
