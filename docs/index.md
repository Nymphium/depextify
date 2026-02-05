---
layout: default
title: depextify Documentation
---

# depextify

`depextify` is a command-line tool designed to analyze shell scripts, Makefiles, Dockerfiles, and other configuration files to extract external command dependencies. It helps developers and packagers (especially Nix users) identify missing dependencies in their environments.

## Installation

### Using Go

If you have Go installed, you can install the latest version directly:

```sh
go install github.com/nymphium/depextify/cmd/depextify@latest
```

### Using Nix

You can run `depextify` without installation using Nix Flakes:

```sh
nix run github:nymphium/depextify -- <args>
```

---

## Usage

```sh
depextify [options] <file|directory>
```

By default, `depextify` recursively scans the specified directory or file. It automatically filters out shell built-ins, GNU coreutils, and other common tools to reduce noise.

### Command Line Options

| Option | Description | Default |
| :--- | :--- | :--- |
| `-count` | Show the occurrence count for each command. | `false` |
| `-pos` | Show file path, line number, and the source line for each occurrence. | `false` |
| `-hidden` | Recursively scan hidden files and directories (e.g., `.git`, `.config`). | `false` |
| `-builtin` | Include shell built-in commands (e.g., `cd`, `echo`, `export`) in the output. | `false` |
| `-coreutils` | Include GNU Coreutils commands (e.g., `ls`, `cp`, `mv`) in the output. | `false` |
| `-common` | Include "common" tools (e.g., `grep`, `sed`, `awk`, `curl`, `git`) in the output. | `false` |
| `-no-builtin` | Explicitly exclude shell built-ins (inverse of `-builtin`). | `true` |
| `-no-coreutils`| Explicitly exclude coreutils (inverse of `-coreutils`). | `true` |
| `-no-common` | Explicitly exclude common tools (inverse of `-common`). | `true` |
| `-color` | Force enable colored output. | `auto` |
| `-no-color` | Force disable colored output. | `false` |
| `-ignores` | Comma-separated list of additional commands to ignore. Example: `-ignores=npm,node` | `""` |
| `-list` | List ignored commands in specified categories and exit. Categories: `builtins`, `coreutils`, `common`, `all`. | `""` |
| `-lexer` | Specify the chroma lexer for highlighting. | `bash` |
| `-style` | Specify the chroma style for highlighting. | `monokai` |
| `-format` | Output format. Options: `text`, `json`, `yaml`. | `text` |

### Environment Variables

| Variable | Description |
| :--- | :--- |
| `DEPEXTIFY_STYLE` | Sets the default syntax highlighting style (e.g., `monokai`, `dracula`). Overridden by `-style` flag. |

---

## Configuration File

`depextify` supports project-specific configuration via a `.depextify.yaml` file. It looks for this file in the **current working directory** first, then in the **user's home directory**.

### `.depextify.yaml` Reference

```yaml
# Control visibility of specific command categories
no_builtins: true   # If true, ignore shell builtins (cd, echo...)
no_coreutils: true  # If true, ignore GNU coreutils (ls, cp...)
no_common: true     # If true, ignore common tools (grep, sed...)

# Output behavior
show_count: false   # Show occurrence counts
show_pos: false     # Show file positions and source lines
use_color: true     # Enable colored output
format: text        # Output format: text, json, yaml

# Syntax highlighting
lexer: bash         # Lexer to use for code snippets
style: monokai      # Chroma style name

# Scan behavior
show_hidden: false  # Scan hidden files/directories

# Custom exclusions
ignores:            # List of command names to ignore globally
  - my-internal-tool
  - npm
  - node

excludes:           # List of file/directory glob patterns to skip
  - vendor/
  - node_modules/
  - dist/
  - "**/*.min.js"
```

---

## Ignore Files

You can exclude specific files or directories from being scanned by placing a `.depextifyignore` file in the project root. The syntax follows standard `.gitignore` rules.

**Example `.depextifyignore`:**

```gitignore
# Ignore dependency directories
node_modules/
vendor/

# Ignore specific file types
*.log
*.tmp

# Ignore specific scripts
scripts/dev-only.sh
```

---

## Supported File Formats

`depextify` detects dependencies in various file formats by parsing the underlying shell scripts embedded within them.

### 1. Shell Scripts
*   **Extensions:** `.sh`, `.bash`, `.zsh`, `.ksh`
*   **Shebangs:** Files starting with `#!/bin/sh`, `#!/bin/bash`, `#!/usr/bin/env bash`, etc.
*   **Parser:** Uses `mvdan.cc/sh` for accurate AST-based parsing.

### 2. Makefiles
*   **Filenames:** `Makefile`, `makefile`, `GNUmakefile`
*   **Logic:** Extracts commands from recipe lines (lines starting with tabs). Handles prefixes like `@`, `-`, and `+`.

### 3. Dockerfiles
*   **Filenames:** `Dockerfile`, `Dockerfile.*`
*   **Logic:** Extracts and parses commands from `RUN` instructions. Supports both single-line and multi-line (backslash-continued) `RUN` commands. Ignores `RUN ["exec", "form"]`.

### 4. GitHub Actions Workflows
*   **Paths:** `.github/workflows/*.yml`, `.github/workflows/*.yaml`
*   **Logic:** Extracts scripts from the `run:` key in steps.

### 5. Taskfiles
*   **Filenames:** `Taskfile.yml`, `Taskfile.yaml`, `taskfile.yml`, `taskfile.yaml`
*   **Logic:** Extracts commands from `cmd:` strings and `cmds:` lists.

---

## Library Usage (Go)

`depextify` is structured to be used as a Go library as well.

```go
import (
    "fmt"
    "github.com/nymphium/depextify/depextify"
)

func main() {
    config := &depextify.Config{
        NoBuiltins:  true,
        NoCoreutils: true,
    }

    // Scan a directory
    results, err := config.Scan(".")
    if err != nil {
        panic(err)
    }

    // Process results
    for cmd, locs := range results.Commands {
        fmt.Printf("Command: %s, Count: %d\n", cmd, len(locs))
    }
}
```

## License

MIT
