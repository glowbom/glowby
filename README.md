# Glowby OSS

Glowby helps you build production-ready software with coding agents. It is an open source coding agent workflow for real projects. It is built primarily for Glowbom projects, but the workflow can also work with other project structures.

## What It Does

- Make software projects and prototypes production-ready with coding agents
- Run on local projects with ChatGPT login, API keys, or OpenCode config

## Install

Install the Glowby CLI:

```bash
curl -fsSL https://raw.githubusercontent.com/glowbom/glowby/main/scripts/install.sh | sudo sh
```

For Windows, we recommend using WSL and running the install command inside Ubuntu.

If you prefer a user-local install directory:

```bash
mkdir -p ~/.local/bin
GLOWBY_INSTALL_DIR="$HOME/.local/bin" curl -fsSL https://raw.githubusercontent.com/glowbom/glowby/main/scripts/install.sh | sh
```

Then clone the repo and enter it:

```bash
git clone https://github.com/glowbom/glowby.git
cd glowby
```

## Quickstart

Glowby needs these tools available on your `PATH`:

- [Go](https://go.dev/)
- [Bun](https://bun.sh/)
- [OpenCode](https://opencode.ai/)

Run the built-in environment check and launch Glowby:

```bash
glowby doctor
glowby code
```

Run those commands from the Glowby repo root, where `backend/` and `web/` live side by side.

## Start Using Glowby OSS

1. Open `http://localhost:4572`
2. Load a local project
3. Choose how you want to run the agent:
   - ChatGPT login
   - API keys
   - OpenCode config
4. Start a refine run

## Requirements And Setup

If `glowby doctor` reports missing tools, install them first and confirm they are available on your `PATH`:

```bash
go version
bun --version
opencode --version
```

If any command is not found, restart your terminal first. If it still does not work, add the tool's install location to your `PATH` or reinstall it using the tool's recommended installer.

On macOS, a common fix is to add the tool's bin directory to your shell profile (usually `~/.zshrc`) and then reload it:

```bash
# Common PATH fixes on macOS
echo 'export PATH="/usr/local/go/bin:$PATH"' >> ~/.zshrc
echo 'export BUN_INSTALL="$HOME/.bun"' >> ~/.zshrc
echo 'export PATH="$BUN_INSTALL/bin:$PATH"' >> ~/.zshrc

# Add the directory that contains the opencode binary
echo 'export PATH="/path/to/opencode/bin:$PATH"' >> ~/.zshrc

source ~/.zshrc
```

If you use Bash instead of zsh, update `~/.bash_profile` or `~/.bashrc` instead.

### Manual fallback

If you prefer to launch the stack without the CLI, run the backend and web app separately:

#### Backend

```bash
cd backend
go run .
```
The backend runs on `http://localhost:4569`.

#### Web app

```bash
cd web
bun install
bun run dev
```

The web app runs on `http://localhost:4572`.

## Using the Bundled Default Project

This repo includes a ready-to-use Glowbom default project in `project/`.

You can use `project/` as your main starting template without logging in to Glowbom.com or downloading a project export first. Just copy the folder, rename it if you want, and start customizing it locally.

The bundled project includes:

- `project/prototype/` - reference design and assets
- `project/apple/` - Apple app project
- `project/android/` - Android app project
- `project/web/` - web app project
- `project/glowbom.json` - project manifest

If you only need some targets, remove the platform folders you do not want:

- Delete `project/apple/` if you do not need Apple platforms
- Delete `project/android/` if you do not need Android
- Delete `project/web/` if you do not need web
- Keep all of them if you want to build every platform in sync from one Glowbom project

## Project Structure

- `backend/` - Go backend
- `project/` - bundled default Glowbom project template
- `web/` - React + Vite web app
- `legacy/` - older Glowby code kept for reference
