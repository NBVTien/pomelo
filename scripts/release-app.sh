#!/usr/bin/env bash
# Build the notarized native app DMG and publish it to pomelo-releases together
# with the Sparkle appcast. Version comes from cmd/pom/root.go and must match
# cmd/libpom appVersion; the tag must already be pushed (run `make patch` first).
# Old releases are never deleted. Set DRY_RUN=1 to build without publishing.
#
# Env overrides (set in Makefile.local): RELEASE_NOTES / RELEASE_NOTES_FILE,
# GH_USER_PUBLISH (gh account to publish as), GH_USER_BACK, RELEASE_REPO, DRY_RUN.
set -euo pipefail
cd "$(dirname "$0")/.."

VERSION=$(grep '^const version' cmd/pom/root.go | cut -d'"' -f2)
APP_VERSION=$(grep '^const appVersion' cmd/libpom/libpom.go | cut -d'"' -f2)
[ "$VERSION" = "$APP_VERSION" ] || { echo "version drift: pom=$VERSION libpom=$APP_VERSION — run make patch" >&2; exit 1; }
TAG="v$VERSION"
DIST="desktop/PomeloApp/dist"
REPO="${RELEASE_REPO:-pomelohq/pomelo}"
GH_USER_PUBLISH="${GH_USER_PUBLISH:-}"   # set in Makefile.local (gh account to publish as)
GH_USER_BACK="${GH_USER_BACK:-}"         # optional: account to switch back to after
if [ -n "${RELEASE_NOTES_FILE:-}" ] && [ -f "${RELEASE_NOTES_FILE}" ]; then
  NOTES="$(cat "$RELEASE_NOTES_FILE")"
elif [ -n "${RELEASE_NOTES:-}" ]; then
  NOTES="${RELEASE_NOTES}"
else
  # Auto: user-facing commit subjects since the previous tag (skip housekeeping).
  PREV_TAG="$(git describe --tags --abbrev=0 "$TAG^" 2>/dev/null || true)"
  if [ -n "$PREV_TAG" ]; then
    NOTES="$(git log --no-merges --pretty=format:'- %s' "$PREV_TAG..$TAG" \
      | grep -viE '^- (chore|refactor|test|ci|style)(\(|:)' || true)"
  fi
  [ -n "${NOTES:-}" ] || NOTES="Pomelo $TAG"
fi
export RELEASE_NOTES="$NOTES"   # package.sh renders it as the appcast <description>

if [ "${DRY_RUN:-0}" != "1" ] && ! git ls-remote --tags origin "$TAG" | grep -q "$TAG"; then
  echo "error: tag $TAG is not on origin — run 'make patch' first (or DRY_RUN=1)" >&2
  exit 1
fi

# Build + sign + notarize + staple the DMG; also (re)writes dist/appcast.xml.
DRY_RUN="${DRY_RUN:-0}" bash desktop/PomeloApp/package.sh

DMG="$DIST/Pomelo-$VERSION.dmg"
[ -f "$DMG" ] || { echo "error: $DMG not built" >&2; exit 1; }
( cd "$DIST" && shasum -a 256 "Pomelo-$VERSION.dmg" appcast.xml > checksums.txt )

if [ "${DRY_RUN:-0}" = "1" ]; then
  echo ">> DRY_RUN=1 — built $DMG (not published)"
  exit 0
fi

[ -n "$GH_USER_PUBLISH" ] || { echo "set GH_USER_PUBLISH (gh account with write access to $REPO) — e.g. in Makefile.local, or DRY_RUN=1 to build only" >&2; exit 1; }
gh auth switch -u "$GH_USER_PUBLISH" >/dev/null
restore() { [ -n "${GH_USER_BACK}" ] && gh auth switch -u "$GH_USER_BACK" >/dev/null 2>&1 || true; }
trap restore EXIT

# appcast.xml is uploaded as a release asset so SUFeedURL
# (/releases/latest/download/appcast.xml) serves it 302 no-cache — the raw.git
# CDN cached it for 5min and caused update lag. Create-or-upload so a half-run
# (release exists, assets missing) still finishes.
ASSETS=("$DMG" "$DIST/appcast.xml" "$DIST/checksums.txt")
if ! gh release create "$TAG" --repo "$REPO" --title "$TAG" --notes "$NOTES" "${ASSETS[@]}" 2>/dev/null; then
  echo ">> $TAG exists — uploading assets (--clobber)"
  gh release upload "$TAG" --repo "$REPO" --clobber "${ASSETS[@]}"
fi

echo ">> done: native app $TAG published to $REPO"
