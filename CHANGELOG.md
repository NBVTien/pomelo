# Changelog

All notable changes to Pomelo are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and Pomelo follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.2] - 2026-08-28

### Added
- Database navigator rebuilt on a native outline view: smooth row reuse for large schemas, correct nested indentation (server > database > tables), native disclosure, and drag-to-reorder that keeps working even with tables expanded. (#59)

### Changed
- The main workspace's "keep fresh" status now shows the real sync outcome and how fresh it is ("synced 2m ago", "N updated", or a failure) instead of a bare countdown; the next-run timer moves into a per-repo popover. Each repo is pulled onto its own default branch, and a pull runs immediately on launch. (#59)
- Consistent loading vs empty states across the PR tabs, Activity, Secrets, and diffs, plus a shared UI kit (spinner, cards, pills, section headers) drawn without native controls for a uniform look. (#59)

### Fixed
- Pull request Files and Commits now match the PR even when the local worktree is behind, by diffing the pushed PR refs. (#59)
- The PR conversation loads reliably (its timeline is now assembled by the core), and PR and Jira details are cached to disk so reopening a workspace is instant. (#59)
- Global keyboard shortcuts (settings, shared services, and friends) fire even while a text field or the SQL editor is focused. (#59)
- Crash log output wraps instead of scrolling sideways. (#59)

## [0.3.1] - 2026-08-28

### Fixed
- A running service no longer loses its allocated port when the process briefly stops accepting connections (e.g. a dev server reloading): the port is only reclaimed after it stays unreachable for a sustained window, so the dev-proxy keeps routing to it instead of falling back to a costly live scan that could spike CPU. (#56)

## [0.3.0] - 2026-08-28

### Added
- Add repos to an existing workspace: right-click a workspace and pick "Add repo..." to fork more repos onto its branch, wiring up their env, ports, and services without touching the repos already there. (#54)

### Changed
- PR and Jira panes are far more readable: a GitHub-style conversation timeline with avatars, review threads, and inline comments, laid out in a centered reading column, plus sepia theme fixes. Review bodies are now surfaced from the core. (#49)

### Fixed
- Shared services now connect over the explicit IPv4 loopback (127.0.0.1) instead of "localhost", avoiding IPv6 connection failures on Docker Desktop where only some clients could reach the port. (#54)
- A service that just crashed no longer briefly shows as running: holder liveness now treats an exited-but-unreaped process as dead. (#55)

## [0.2.5] - 2026-08-27

### Fixed
- Diff viewer: long lines scroll horizontally in both unified and split views instead of getting cut off, and the added/removed tint spans the full line. (#48)
- Workspace sidebar scrolls smoothly; a fast flick no longer leaves a phantom blank gap below the list. (#50)
- Shared services now hand out a connection URL on the port the container is actually published on, including capacity>1 services where a workspace is pinned to a second instance. (#50, #51)
- Creating a workspace whose repo selection matches nothing now fails with a clear message instead of leaving an empty, unusable workspace. (#50)
- Dependency board shows only the current project's caches and fits to the window when opened. (#48)
- Diff view renders renames compactly and groups file-tree paths correctly. (#36)

### Changed
- Internal: the core-to-UI boundary is now data-routed through a few verbs (query/command/fetch/subscribe) instead of ~86 typed exports, and domain logic (PR status, diff parsing, dependency ordering, agent notifications) lives in the Go core per ADR 0001. No user-facing behavior change. (#39, #45, #48, #50, #52)

## [0.2.4] - 2026-08-27

### Fixed
- Keeping the golden source fresh now updates a main that has diverged from origin (upstream rebase or force-push) by mirroring origin, instead of silently skipping it. (#37)

### Changed
- Internal: standardized the core-to-UI boundary with DTO contract tests and a view-model data layer; no user-facing behavior change. (#35)

## [0.2.3] - 2026-08-27

### Fixed
- Opening a service in the browser now builds a URL that resolves even when two workspaces share a ticket prefix; it falls back to the full branch host instead of an ambiguous short one. (#34)

## [0.2.2] - 2026-08-27

### Added
- Claude usage meter in the top bar: 5h session and weekly windows with a compact bar, color-coded by load. (#32)
- PR view: a file tree with a local-changes sidebar, and tooltips across the app. (#33)

### Changed
- The compacting agent state now has its own distinct pulsing orb instead of a plain grey one. (#31)

## [0.2.1] - 2026-08-27

### Added
- Notification sounds: pick a sound per event (or several, played at random), save them as switchable sound sets, and upload your own audio. Delivery has a master toggle and an option to alert even while you're viewing the workspace. (#29)

### Fixed
- The Database pane shortcut (Cmd-4) is now listed in Settings > Shortcuts. (#30)

## [0.2.0] - 2026-08-26

### Added
- Dependency Store: a global node_modules cache board (node_modules -> hash -> workspaces) with Optimize to capture hand-installed deps and Dedupe to reclaim disk via CoW. (#27)
- Update from origin for the golden source: a main-only action with a per-repo progress sheet. (#27)

### Changed
- Workspace, repo-column, and database-tree reordering is gesture-driven now. (#27)
- CI is path-gated behind a single CI Gate; onboarding seeds pom.yml via detect. (#27)

### Fixed
- Config tree fits the sidebar width instead of clipping long fragment names. (#27)
- Golden-source update pulls only the default branch, avoiding the fast-forward-to-multiple-branches failure. (#27)

## [0.1.7] - 2026-08-26

### Added
- Optimistic, animated start/stop for a repo's services — cards flip to an
  immediate starting…/stopping… state and animate between states. (#20)

### Changed
- Create workspace: the sprint picker uses a themed dropdown, and the ticket
  suggestions render as a solid card with hover, close on pick, and no longer
  overlap the hint. (#21)

### Fixed
- A stopped service could keep showing "running" after the OS recycled the dead
  holder's pid; the pidfile is now removed on kill. (#19)
- Quit no longer hangs while tearing down a workspace's ephemeral shells. (#18)

## [0.1.6] - 2026-08-26

### Added
- Jira pane: a read-only "Web links" section listing an issue's remote links. (#16)
- App icon shown in the session chip and the create-workspace sheet. (#11)

### Changed
- Shortcuts keep their tab and output open after finishing (Ctrl+D to close). (#13)
- Holder lifecycle unified behind a single interface; shells receive injected
  env instead of sourcing a hardcoded .env.local. (#15)

### Fixed
- Activity Monitor no longer blanks a workspace's process group. (#14)
- Attaching to a stopped service waits for its holder instead of spawning a bare
  shell over it, and shells are no longer reaped on app launch. (#10)
- The port reaper only reclaims ports Pomelo allocated — never a user's own
  running process. (#9)
- Jira tickets with zero comments now render instead of failing to load. (#16)

### Build
- Styled DMG installer window, built headlessly for CI. (#12)

## [0.1.5] - 2026-08-25

### Fixed
- Re-derive the description, name, and slug when the Jira ticket changes. (#8)

## [0.1.4] - 2026-08-25

### Changed
- Point the in-app update feed at the pomelohq/pomelo releases. (#7)

### Documentation
- README with a hero image, app screenshot, and architecture diagram. (#6)

## [0.1.3] - 2026-08-25

### Changed
- One unified "New session" flow; renamed Project to Session. (#5)

## [0.1.2] - 2026-08-25

### Changed
- Purged legacy branding; fixed the session root and session switching. (#4)

## [0.1.1] - 2026-08-25

### Added
- "Open a project…" entry in the session dropdown. (#2)
