# lockbox

## Project Overview

`lockbox` is a Go CLI tool for secure secret management on headless Linux
systems. It combines age-encrypted persistent storage with Linux kernel keyring
session caching to provide an ssh-agent-style experience for PATs and other
secrets.

See [PLAN.md](PLAN.md) for the full design document.

## Build and Test

```bash
make build        # compile binary with version info
make test         # run all tests
make vet          # run go vet
make check        # vet + test (use before committing)
```

Or directly:

```bash
go build ./...
go test ./...
go vet ./...
```

## Project Structure

- `cmd/lockbox/main.go` -- CLI entrypoint
- `internal/store/` -- Encrypted store (age-based, single file)
- `internal/cache/` -- SessionCache interface + Linux kernel keyring backend
- `internal/cmd/` -- Cobra command implementations

## Key Conventions

- **Language**: Go
- **Encryption**: `filippo.io/age` with passphrase-based scrypt KDF
- **Session cache**: Linux kernel keyring via `keyctl` syscalls (`golang.org/x/sys/unix`)
- **CLI framework**: `github.com/spf13/cobra`
- **Platform support**: Linux first. macOS/Windows backends planned behind the
  `SessionCache` interface. Use build tags (`keyring_linux.go`,
  `keyring_stub.go`) for platform-specific code.
- **Error handling**: Follow Go idioms. Return errors up the call stack; handle
  them at the command level with user-friendly messages. Do not panic.
- **File permissions**: Store file 0600, store directory 0700. Enforce on
  creation and verify on open.
- **Secret hygiene**: Never log, print, or include secret values in error
  messages. Secret values are only output by explicit commands (`get`, `run`,
  `env`).
- **No comments unless necessary**: Keep code self-documenting. Use comments
  for non-obvious security decisions or platform-specific gotchas, not for
  restating what the code does.
- **Testing**: Unit tests alongside source files. Integration tests that
  exercise the kernel keyring should be guarded by build tags or runtime
  checks so they can run on Linux CI without breaking other platforms.

## Verification Checklist

After making changes, run:

```bash
make check
```

This runs `go vet` and `go test`. All must pass before considering a change
complete.

## Versioning

This project uses **semantic versioning** (semver). The version is injected
at build time via `-ldflags` (see `Makefile` and `.goreleaser.yml`).

The version variables live in `internal/cmd/version.go`:

```go
var (
    version = "dev"
    commit  = "none"
    date    = "unknown"
)
```

These are overwritten by `make build` and `goreleaser` -- do NOT hardcode
version strings anywhere else.

### When to bump the version

- **Patch** (0.1.x): Bug fixes, documentation changes, internal refactors
  with no user-facing behavior change.
- **Minor** (0.x.0): New features, new commands, new flags, backwards-compatible
  changes to existing behavior.
- **Major** (x.0.0): Breaking changes to CLI interface, store format changes
  that require migration, removal of commands or flags.

While the project is at 0.x.y, minor version bumps may include breaking
changes (per semver convention for pre-1.0).

### How to release

```bash
make release V=0.2.0
```

This will:
1. Check that the working tree is clean
2. Create and push a git tag `v0.2.0`
3. Run `goreleaser` to build binaries and create a GitHub release

For local testing without publishing:

```bash
make release-snapshot
```

## Store Location

Default: `~/.config/lockbox/store.age`

## Dependencies

- `filippo.io/age`
- `golang.org/x/sys/unix`
- `golang.org/x/term`
- `github.com/spf13/cobra`
