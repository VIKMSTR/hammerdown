# CLAUDE.md

Guidance for Claude Code when working in this repository.
Shared instructions for all coding agents live in @AGENTS.md.

## Commands
- Build: `make`
- Test everything: `make test`
- Run one test: `make test RUN=TestName`

## Architecture
- `cmd/` holds the entry points.
- `internal/` holds the application packages; keep them free of UI code.

## Workflow
- Read the relevant code before changing it; make small, focused edits.
- Run the tests after each change and report failures honestly.
- Ask before destructive actions: deleting files, force pushes, migrations.

## Gotchas
- List the non-obvious pitfalls here, e.g. generated files that must not be edited by hand.
