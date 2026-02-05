# Future Improvements & Ideas

## 1. Output Format Diversification (Enhanced Integration/Automation)
- [x] **JSON / YAML Output**
    - **Reason**: Essential for filtering with `jq`, importing into dashboard tools, and automated checks in CI pipelines.
    - **Content**: Structured data containing dependent commands per file, line numbers, occurrence counts, etc.
- [ ] **Markdown Output (`--format=markdown`)**
    - **Reason**: Output in a format that can be directly pasted into `README.md` or development docs as a "Dependency List".

## 2. Configuration Persistence & Sharing (Enhanced Team Development)
- [x] **Configuration File Support (`.depextify.yaml` / `depextify.toml`)**
    - **Reason**: Share ignored commands, excluded directories, and default styles across the team. git-managed.
- [x] **`.depextifyignore` File**
    - **Reason**: Dedicated file to specify files/directories to exclude from analysis, similar to `.gitignore`.

## 3. Package Manager Integration (Enhanced Practicality)
- [ ] **Installer Generation (`--generate`)**
    - **Reason**: Generate installation script templates based on detected commands.
    - **Examples**:
        - `--generate=brew` -> `Brewfile` format.
        - `--generate=apt` -> `apt-get install ...` string.
        - `--generate=nix` -> `shell.nix` / `flake.nix` package candidates.

## 4. Policy Enforcement (Security & Quality Control)
- [ ] **Allow/Deny Lists (`--allow`, `--deny`)**
    - **Reason**: Detect and error on security-risk commands (e.g., `nc`, `telnet`, `sudo`) or prohibited tools.
- [ ] **Version/Variant Hints**
    - **Reason**: Warn about differences like `python3` vs `python2` or `grep` (BSD) vs `ggrep` (GNU).

## 5. Expanded Analysis Targets
- [ ] **`Makefile` / `Justfile` / `Taskfile` Analysis**
    - **Reason**: External commands are heavily used in build tools; covering them reveals project-wide dependencies.
- [ ] **`Dockerfile` Analysis**
    - **Reason**: Extract commands used in `RUN` instructions.
- [ ] **GitHub Actions (`.github/workflows/*.yml`) Analysis**
    - **Reason**: Extract commands used in `run:` steps.

## 6. UX Improvements
- [ ] **Interactive Mode (`--interactive` / `-i`)**
    - **Reason**: Interactively add unknown commands to ignore lists or categorize them.
- [ ] **Command Lookup (Experimental)**
    - **Reason**: Search online/local DBs to find which package contains the command (e.g., `dig` -> `bind-utils`).
