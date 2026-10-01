# Contributing

## Branches

- **`dev`** is where work happens. Commit or merge feature branches into `dev`.
- **`main`** is what's released. It only changes through a pull request from `dev`, and only when CI passes.

## Commit messages

paceline-desktop uses [Conventional Commits](https://www.conventionalcommits.org), the same as paceline: `feat:` for a new feature, `fix:` for a bug fix, and `docs:`, `test:`, `ci:`, `chore:`, or `refactor:` for everything else. Once releases are automated (milestone 6 in the [design](docs/design.md#milestones)), these types choose the version number.

Messages are checked by a local `commit-msg` hook, installed when you run `npm install`, and by CI on every pull request.

## Development

paceline-desktop is Go (1.26 or later). Node.js runs only commitlint.

```sh
npm install            # commitlint and the commit-msg hook
go test ./...          # the full suite
go test -race ./...    # with the race detector
go vet ./...
gofmt -l .             # lists unformatted files; gofmt -w . fixes them
```

CI also runs [golangci-lint](https://golangci-lint.run) (config in `.golangci.yml`), `govulncheck`, and a 90% coverage floor.

The code layout follows Go convention. `cmd/` holds the binaries, and `internal/` holds one package per concern. Tests sit next to the code they cover, as `*_test.go`. The [design](docs/design.md#package-layout) lists every planned package.

### Policy rules

`internal/policy` holds repository-wide rules that are tested like behavior:

- **Restricted imports.** Network packages are allowed only in `internal/usage/oauth`, and `os/exec` only in `internal/creds`. `unsafe`, `plugin`, and `syscall` are not allowed anywhere.
- **Allowed modules.** Every third-party module in `go.mod` must be listed in `allowedModules` with the reason it's needed.
- **ASCII source.** Go files must be ASCII. Write any other character as a `\u` escape.

Widening any of these is a security decision. Explain it in the pull request.
