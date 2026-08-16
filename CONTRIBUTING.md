# Contributing

AI Server Shell is currently defining its first protocol profile. Contributions should preserve the boundary between the protocol shell and application behavior.

## Before opening a change

- Use an issue for protocol additions or public interface changes.
- Keep model providers, agent loops, memory systems, and product logic outside this repository.
- Include a compatibility or conformance test for wire-level behavior.
- Document deviations from the corresponding OpenAI API behavior.

## Development

Requirements:

- Go 1.24 or later
- Node.js only for official JavaScript client conformance tests once they are added

Run the Go checks:

```sh
go test ./...
go test -race ./...
go vet ./...
```

## Pull requests

A pull request should:

1. Explain the protocol behavior being added or changed.
2. Identify the compatibility profile and affected events or routes.
3. Include tests that fail without the change.
4. Keep public interfaces minimal.
5. Update documentation and the compatibility matrix when applicable.

## Commit messages

Use a short imperative title. Add a body when the compatibility impact or design rationale is not obvious from the diff.

## Code of conduct

Participation is governed by [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md).
