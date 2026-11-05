# AGENTS.md

## Project overview
Describe in one or two sentences what this project does and who uses it.

## Setup and commands
```sh
make          # build
make test     # run all tests
make check    # lint + tests, run before every commit
```

## Code style
- Follow the patterns already used in the file you are editing.
- Keep functions small and names descriptive; comment the *why*, not the *what*.
- Do not reformat code you did not change.

## Testing
- Add or update a test for every behaviour change and bug fix.
- All tests must pass before you finish a task.

## Boundaries
- Never commit secrets, tokens or `.env` files.
- Ask before adding a dependency or changing the public API.
