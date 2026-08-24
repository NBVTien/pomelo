# Pomelo

[![Release](https://img.shields.io/github/v/release/pomelohq/pomelo)](https://github.com/pomelohq/pomelo/releases/latest)
[![License: AGPL v3](https://img.shields.io/badge/License-AGPL_v3-blue.svg)](https://www.gnu.org/licenses/agpl-3.0)
[![Platform: macOS](https://img.shields.io/badge/platform-macOS%2014%2B%20·%20Apple%20Silicon-lightgrey.svg)](#install)

**A dev environment. One per branch.**

Pomelo is a native macOS app that spins up a full, isolated, runnable environment
for every branch of a multi-repo project — services, databases, and shared
infrastructure, wired automatically. No YAML archaeology, no port juggling.

- **One branch, one full stack.** Every branch is a real git worktree with its own
  services, ports, and databases. Two branches never collide.
- **Fast switches.** New workspaces clone databases from a prepared `main`
  (`CREATE DATABASE … TEMPLATE`) and materialize `node_modules` via APFS
  copy-on-write — seconds, not rebuilds.
- **Native services, Docker only for data.** Your repos' services run as real
  native processes on self-managed PTY holders — lighter and faster than
  wrapping everything in containers. Only databases and shared infra
  (Postgres/Redis/MinIO/OpenSearch) run in Docker.
- **Native app, not Electron.** SwiftUI linking a Go core in-process over FFI —
  no localhost server, built for ProMotion.
- **In-app database browser.** Inspect and query each branch's Postgres/Redis
  without leaving the app.
- **Agent-ready.** A built-in Claude agent per workspace, wired to the environment
  it's working in via MCP tools.

## Why Pomelo

Hand-writing code is no longer the bottleneck — knowing a change is *correct* is.
That needs ground truth, not another opinion. Pomelo's moat is that **every branch
has a real, running environment**: DB, services, ports, resolved env. So when you
(or an AI agent) make a change, you can **run it and know** — not guess from a
diff.

The loop: **you reason → the agent types → the env proves → you judge.** Two
control points stay human — the shape of the commit/PR, and judging review
feedback. The agent never owns feature logic, and its output is suspect until the
env proves it.

## Install

Download the latest signed, notarized DMG from
[Releases](https://github.com/pomelohq/pomelo/releases/latest),
drag **Pomelo** into **Applications**, and open it. macOS 14+ · Apple Silicon.

Full docs: **https://pom.toantran292.net/**

## Build from source

Requires Go 1.26+, Xcode, and `zsh`.

```bash
# CLI / Go core
make build            # -> ./pom

# Native macOS app (Go c-archive -> SwiftPM/xcodebuild)
cd desktop/PomeloApp && ./build.sh
```

See [`RELEASE.md`](RELEASE.md) for the packaging / notarization flow.

## Architecture

A transport-neutral Go core (`internal/`) runs dev services on self-managed PTY
holders (tmux-free) and is consumed two ways: the **native app**
(`desktop/PomeloApp`) links it via the `libpom` c-archive FFI, and the `pom` CLI
drives it directly. Config lives in a `pom.yml` (plus an optional `pom.d/` of
fragments) found by walking up from the current directory.

## Contributing

Contributions are welcome. See [`CONTRIBUTING.md`](CONTRIBUTING.md) for the
build/test workflow, code style, and the DCO sign-off we require on every commit.
By participating you agree to the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Copyright (C) 2026 Toan Tran.

Pomelo is free software licensed under the [GNU AGPL-3.0](LICENSE). Commercial
licensing is available from the copyright holder.
