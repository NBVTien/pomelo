# Release process

**The release is the native macOS app.** The app is self-contained — it re-execs
its own bundle binary for the `pty` / `mcp` / `prepare-main` subcommands
(`pombin.Path() = os.Executable()`), so it needs **no external `pom` CLI installed**.
There is no separate CLI distribution anymore (the old `make publish` / Homebrew
tap / `tncli-releases` bridge / `pom update` self-update / `pom daemon` were all
removed in v0.10.9).

One version covers both the app and the in-tree CLI: same `vMAJOR.MINOR.PATCH`
(the two constants stay in lockstep).

## Version naming (semver)

- **patch** (`0.10.8 → 0.10.9`) — bug fix, no new user-facing surface.
- **minor** (`0.10.8 → 0.11.0`) — new feature / new surface, backward-compatible.
- **major** (`0.10.8 → 1.0.0`) — breaking change (config schema, CLI, storage).

Tag is `v<version>` (e.g. `v0.10.9`). The version lives in **two** constants that
must always match — `make patch/minor/major` bumps both and `make version-check`
guards against drift:

- `cmd/pom/root.go` `const version`
- `cmd/libpom/libpom.go` `const appVersion` — drives the DMG name + Sparkle appcast.

## Steps

1. Land your changes on `main` (green: `go build ./... && go vet ./... && go test ./...`;
   app touched → `swift build && swift test` in `desktop/PomeloApp`).
2. **Bump + tag + push:** `make patch` (or `minor` / `major`). Bumps both consts,
   commits `release: v<x>`, tags `v<x>`, pushes to `main`.
3. **Publish the native app:** `make app-publish` — builds the notarized DMG,
   writes the Sparkle appcast, and creates the `pomelo-releases` GitHub release with
   `Pomelo-<x>.dmg` + `appcast.xml` + `checksums.txt`. Switches gh accounts around
   the release and keeps old releases.
4. Bump the docs landing version in `pomelo-docs` (`.vitepress/theme/PomHome.vue`
   eyebrow), `npm run build`, push.

Dry run: `make app-publish DRY_RUN=1` builds the DMG without creating a release.

## Rules

- Never delete old GitHub releases — keep the full history.
- Auto-update feed (0.10.8+) is `…/releases/latest/download/appcast.xml` (302,
  no-cache); the `latest` release must carry the `appcast.xml` asset. Older installs
  (≤0.10.7) read `appcast.xml` off `pomelo-releases` main — `release-app.sh` keeps
  both in sync so stragglers can still hop forward.
- The Sparkle EdDSA private key lives in the macOS keychain — do not lose it;
  without it `sign_update` can't sign the DMG and auto-update breaks.
