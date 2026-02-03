# Project Environment

This project uses **Nix** and **direnv** for environment management.
- `flake.nix` defines the development shell and dependencies.
- `.envrc` automatically loads the Nix environment when entering the directory.

# depextify Improvement Proposals

Here are some ideas for improving `depextify`:

### Completed Improvements

- [x] **Externalize and Refine the Built-in Command List**: Moved to `depextify/lists.go` and expanded with zsh built-ins and common tools.
- [x] **Add Line Number Information**: Implemented via the `-pos` flag, which also includes line content and syntax highlighting.
