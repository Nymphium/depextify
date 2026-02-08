# Project Environment

This project uses **Nix** and **direnv** for environment management and reproducible builds.
- `flake.nix` defines the development shell, dependencies, and the project's Nix derivation.
- `gomod2nix.toml` handles Go module dependencies for Nix.
- `.envrc` automatically loads the Nix environment when entering the directory.

The tool can be built or run directly using Nix:
```sh
nix build .#default
nix run . -- examples/test.sh
```

`gomod2nix.toml` must be updated after `go.mod` is updated:
```sh
$ gomod2nix
```



# Development Workflow



When you have completed your changes, always run the linter and tests to ensure code quality:

```sh

golangci-lint run

gotest ./...

```



# Documentation



When you add or modify features in `AGENTS.md` or the codebase, you MUST update:

1. `README.md`

2. `docs/` directory (run `depextify --help` or update relevant markdown files)
