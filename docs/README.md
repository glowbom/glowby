# Glowby OSS

Glowby helps you build production-ready software with coding agents. It is an open source coding agent workflow for real projects. It is built primarily for Glowbom projects, but the workflow can also work with other project structures.

## What It Does

- Make software projects and prototypes production-ready with coding agents
- Run on local projects with ChatGPT login, API keys, or OpenCode config

## Requirements

Install these first:

- [Go](https://go.dev/)
- [Bun](https://bun.sh/)
- [OpenCode](https://opencode.ai/)

## Quickstart

### 1. Run the backend

```bash
cd backend
go run .
```
The backend runs on `http://localhost:4569`.

### 2. Run the web app

```bash
cd web
bun install
bun run dev
```

The web app runs on `http://localhost:4572`.

### 3. Start using Glowby OSS

1. Open `http://localhost:4572`
2. Load a local project
3. Choose how you want to run the agent:
   - ChatGPT login
   - API keys
   - OpenCode config
4. Start a refine run

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
