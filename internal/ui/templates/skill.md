---
name: release-notes
description: Drafts release notes from the commits since the last git tag. Use when the user asks for release notes, a changelog entry or a summary of what changed in a release.
---

# Release notes

## Steps
1. Find the latest tag: `git describe --tags --abbrev=0`.
2. List the commits since then: `git log <tag>..HEAD --oneline`.
3. Group them into **Features**, **Fixes** and **Other**; skip merges and version bumps.
4. Write one short, user-facing line per change.

## Output format
```markdown
## v1.2.0
### Features
- Templates for AGENTS.md, CLAUDE.md and skills
### Fixes
- Windows build no longer fails to package
```
