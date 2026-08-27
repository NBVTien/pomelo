# Changelog

All notable changes to Pomelo are documented here. The format is based on
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and Pomelo follows
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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
