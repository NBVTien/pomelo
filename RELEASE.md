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

## Steps (CI — the default)

1. Land your changes on `main` (green: `go build ./... && go vet ./... && go test ./...`;
   app touched → `swift build && swift test` in `desktop/PomeloApp`).
2. **One command:** `make patch` (or `minor` / `major`). Bumps both consts, commits
   `release: v<x>`, tags `v<x>`, pushes to `main`.
3. That's it — pushing the tag triggers **`.github/workflows/release.yml`** on a
   GitHub-hosted macOS runner: build → sign → notarize → DMG → Sparkle appcast →
   publish the `pomelohq/pomelo` GitHub Release with `Pomelo-<x>.dmg` +
   `appcast.xml` + `checksums.txt`. Watch it: `gh run watch --repo pomelohq/pomelo`.
4. Bump the docs landing version in `pomelo-docs` (`.vitepress/theme/PomHome.vue`
   eyebrow), `npm run build`, push.

CI needs these repo secrets (Settings ▸ Secrets ▸ Actions): `MACOS_CERT_P12`
(base64 .p12), `MACOS_CERT_PASSWORD`, `MACOS_SIGN_IDENTITY`, `KEYCHAIN_PASSWORD`,
`NOTARY_APPLE_ID`, `NOTARY_APP_PASSWORD`, `NOTARY_TEAM_ID`, `SPARKLE_ED_PRIVATE_KEY`.

## Local fallback (no CI)

`make app-publish` runs the same `package.sh` on your Mac (uses the local
`pomelo-notary` keychain profile + the keychain EdDSA key) and publishes to
`pomelohq/pomelo` via `gh`. `make app-publish DRY_RUN=1` builds the DMG without
publishing.

## Rules

- Never delete old GitHub releases — keep the full history.
- Auto-update feed is `…/releases/latest/download/appcast.xml` (302, no-cache) on
  `pomelohq/pomelo`; the `latest` release must carry the `appcast.xml` asset.
- The Sparkle EdDSA private key: locally in the macOS keychain, in CI as the
  `SPARKLE_ED_PRIVATE_KEY` secret — do not lose it; without it `sign_update` can't
  sign the DMG and auto-update breaks.
