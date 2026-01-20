# depextify Improvement Proposals

Here are some ideas for improving `depextify`:

### Refactoring Proposal: Externalize and Refine the Built-in Command List

The current implementation uses a hardcoded list of shell built-in commands and keywords. This is inflexible and difficult to maintain.

*   **Problem**: The list is static, potentially incomplete for different shells (bash, zsh, etc.), and mixes keywords (like `if`, `for`) with actual commands (like `cd`, `echo`). The parser already handles keywords, making their inclusion redundant.
*   **Solution**:
    1.  Refine the list to only include command-like built-ins, removing syntactic keywords.
    2.  Externalize this list into a configuration file (e.g., `builtins.txt`). The application would load this list at runtime.
*   **Benefits**: This would make the tool more accurate and flexible, allowing users to provide their own lists of built-ins tailored to a specific shell environment.

### New Feature Proposal: Add Line Number Information

The tool currently only lists the names of the commands, not where they are used.

*   **Feature**: Introduce a command-line flag (e.g., `-n` or `--line-numbers`) to display the line number(s) where each command appears. For example, the output could change from `jq` to `jq:26`.
*   **Implementation**:
    1.  The `mvdan.cc/sh/v3` parser provides position information (line and column) for each node in the AST.
    2.  Modify the `collectCommands` function in `depextify/depextify.go` to store not just the command name, but also the line number. The return type could change from `map[string]struct{}` to `map[string][]int`.
    3.  Update the `main` function in `main.go` to parse the new flag and format the output to include the line numbers.
*   **Benefits**: This feature would significantly improve the tool's utility for auditing, debugging, and refactoring shell scripts, as users could quickly navigate to the exact location of any dependency.
