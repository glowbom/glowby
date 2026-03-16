# glowby CLI

Terminal-first CLI for Glowby OSS. Starts the Go backend and web UI, opens the browser, and manages the full local dev workflow from one command.

## Install

### From GitHub Releases

```sh
curl -fsSL https://raw.githubusercontent.com/glowbom/glowby/main/scripts/install.sh | sh
```

Or set a custom install directory:

```sh
GLOWBY_INSTALL_DIR=~/.local/bin curl -fsSL ... | sh
```

### Build from source

```sh
cd cli
go build -o glowby .
```

## Commands

### `glowby code`

Start the backend, web UI, and open the browser from a local Glowby checkout.

```sh
glowby code                    # Start Glowby from the current checkout
glowby code /path/to/project   # Start Glowby and print a project path hint
```

What it does:
1. Starts the Go backend (`go run .` in `backend/`)
2. Runs `bun install` in `web/` if `node_modules/` is missing
3. Reclaims ports `4569` and `4572` if they are already occupied by a previous Glowby run
4. Starts the web dev server (`bun run dev` in `web/`)
5. Waits for the web server to be ready, then opens the browser
6. If a project path is given, prints the path so you can load it in the UI

Press Ctrl+C to stop both servers.

**Argument parsing:**
- If a positional arg is given and it is an existing directory, it is treated as the project path
- If it is not an existing directory, the command exits with an error

**Finding the Glowby root:** The CLI looks for sibling `backend/` and `web/` directories relative to the binary location or the current working directory and its parent directories.

### `glowby doctor`

Check that required tools are installed.

```sh
glowby doctor
```

Checks for: `go` (required), `bun` (required), `opencode` (required), and a local Glowby checkout with sibling `backend/` and `web/` directories. Returns exit code 1 if required dependencies are missing.

### `glowby version`

```sh
glowby version
```

Prints version, commit hash, and build date. These are injected at build time via ldflags.

## Local Development

```sh
cd cli

# Build
go build -o glowby .

# Build with version info
go build -ldflags "-s -w \
  -X main.version=v0.1.0 \
  -X main.commit=$(git rev-parse --short HEAD) \
  -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o glowby .

# Run
./glowby version
./glowby doctor
./glowby code

# Vet
go vet ./...
```

## Releases

Releases are built automatically by GitHub Actions when a tag matching `v*` is pushed.

```sh
git tag v0.1.0
git push origin v0.1.0
```

The workflow builds binaries for:
- macOS (amd64, arm64)
- Linux (amd64, arm64)
- Windows (amd64, arm64)

Archives are uploaded to the GitHub Release page.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Command or dependency failure |
| 2 | Usage error (unknown command, bad args) |
