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
- Node.js 22 or later for official SDK compatibility tests

Run the Go checks:

```sh
go test ./...
go test -race ./...
go vet ./...
```

Run the complete credential-free gate with `make verify`. Changes to the frozen
OpenAI profile must update the vendored source, lock digests, generated route
manifest, event inventory, compatibility documentation, and official-SDK tests
in the same pull request. `go generate ./...` must then leave no diff.

Live compatibility is a separate opt-in gate. Use the safe profile for
read-only checks. Never run the full profile without a disposable project,
explicit mutation/cost approval, and verified cleanup.

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
