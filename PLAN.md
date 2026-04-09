# lockbox - Design Plan

## Problem Statement

Tools that talk to GitHub, Jira, Confluence, etc. need authentication via
Personal Access Tokens (PATs). These tokens grant access privileges that must
be protected. On headless Linux systems accessed via SSH, there is no
satisfactory way to store and use these secrets securely:

- **GNOME Keyring / KWallet**: Fall back to plaintext storage without a GUI
  session and PAM integration.
- **keepassxc-cli**: Encrypted at rest but requires master password on every
  access -- no session caching.
- **HashiCorp Vault**: Requires running a server. Overkill for personal use.
- **1Password CLI (`op`) / Bitwarden CLI (`bw`)**: Require third-party service
  subscriptions.
- **`pass` + `gpg-agent`**: Closest existing answer. Encrypted at rest, session
  caching via gpg-agent. But GPG setup is notoriously painful and the ecosystem
  is heavy.
- **Linux kernel keyring (`keyctl`)**: Secrets in kernel memory, never on
  filesystem. But no persistence -- secrets lost when session ends. No
  encrypted backing store.

No single existing tool combines:
1. Encrypted at rest (never plaintext on filesystem)
2. Session-based caching (unlock once, available until lock/logout)
3. Pure CLI, works headless over SSH, no external service dependency

## Solution

`lockbox` is a Go CLI tool that bridges encrypted persistent storage with
in-memory session caching, providing an ssh-agent-style experience for
arbitrary secrets.

## Architecture

```
+--------------------------------------------------+
|                    CLI (cobra)                    |
+--------------------------------------------------+
|  init | add | remove | list                      |
|  unlock | lock | get | run | env                 |
+--------------------------------------------------+
|              SessionCache Interface               |
|  Store(name, value) | Retrieve(name) | Clear()   |
+-------------+--------------+---------------------+
| Linux:      | macOS:       | Windows:             |
| keyctl      | Keychain     | Cred Manager         |
| syscalls    | (future)     | (future)             |
+-------------+--------------+---------------------+
                      |
     Encrypted Store (~/.config/lockbox/store.age)
     Single age-encrypted JSON blob
     Passphrase-based encryption (scrypt)
```

### Two-Layer Design

1. **Encrypted store on disk** -- A single `age`-encrypted file containing all
   secrets as JSON. Encrypted with a master password using scrypt KDF. Secrets
   never exist as plaintext on the filesystem.

2. **Session cache in kernel memory** -- Linux kernel keyring via `keyctl`
   syscalls. Secrets live in kernel space. The user keyring (`@u`) is shared
   across all sessions for the same UID and persists as long as the user has
   at least one active session. No daemon needed.

The `unlock` command bridges the two layers: enter master password once per
session, decrypt the store, load secrets into the kernel keyring.

## Encrypted Store Format

Single file at `~/.config/lockbox/store.age` (perms 0600, directory 0700).

Decrypted contents:

```json
{
  "secrets": {
    "github_pat": {
      "value": "ghp_xxxxxxxxxxxx",
      "env_var": "GITHUB_TOKEN",
      "description": "GitHub Personal Access Token",
      "created_at": "2024-01-15T10:30:00Z"
    },
    "jira_token": {
      "value": "ATATT3xFf...",
      "env_var": "JIRA_API_TOKEN",
      "description": "Jira Cloud API Token",
      "created_at": "2024-01-15T10:35:00Z"
    }
  }
}
```

Each secret has:
- **name** (map key) -- human-friendly identifier
- **value** -- the secret itself
- **env_var** -- environment variable name for `run`/`env` commands
- **description** -- optional human-readable note
- **created_at** -- timestamp

Duplicate `env_var` mappings are rejected at `add` time.

## CLI Commands

### `lockbox init`
Create a new encrypted store. Prompts for master password (with confirmation).

### `lockbox add <name> --env VAR [--desc TEXT]`
Add a secret. Prompts for the secret value (no echo). Prompts for master
password to decrypt/re-encrypt the store. Rejects duplicate names or env_var
mappings.

### `lockbox remove <name>`
Remove a secret from the store. Prompts for master password.

### `lockbox list`
List secret names, env var mappings, and descriptions. No values shown.
Prompts for master password (reads from store, not cache, to show all secrets
including any not yet loaded).

### `lockbox unlock`
Decrypt the store and load all secrets into the kernel keyring session cache.
Prompts for master password. Reports how many secrets were loaded.

### `lockbox lock`
Clear all lockbox secrets from the kernel keyring.

### `lockbox get <name>`
Retrieve a single secret value to stdout. Reads from session cache if
unlocked; otherwise falls back to decrypting the store (prompts for master
password).

### `lockbox run [--secrets name,...] -- cmd [args...]`
Execute `cmd` with secrets injected as environment variables into the child
process only. The secrets never appear in the parent shell's environment.

- With no `--secrets` flag: injects all secrets.
- With `--secrets`: injects only the named subset.
- Reads from session cache if unlocked; falls back to decrypt-on-the-fly.

### `lockbox env [--secrets name,...]`
Print `export KEY=VALUE` lines suitable for `eval $(lockbox env)`. Less
secure than `run` (secrets persist in the shell environment) but convenient
for interactive use.

## User Workflow

```bash
# --- First-time setup ---
$ lockbox init
Enter master password: ****
Confirm master password: ****
Store created at ~/.config/lockbox/store.age

# --- Add secrets ---
$ lockbox add github_pat --env GITHUB_TOKEN --desc "GitHub PAT"
Enter secret value: ****
Enter master password: ****
Secret 'github_pat' added.

# --- Start of SSH session ---
$ lockbox unlock
Enter master password: ****
3 secrets loaded into session cache.

# --- Most secure: ephemeral env vars ---
$ lockbox run -- gh pr list
# gh sees GITHUB_TOKEN; it only exists in the child process

# --- Selective injection ---
$ lockbox run --secrets github_pat,jira_token -- my-script.sh

# --- Less secure but convenient ---
$ eval $(lockbox env)
# GITHUB_TOKEN now in current shell environment

# --- End of session ---
$ lockbox lock          # explicit clear
# ...or just disconnect -- session keyring destroyed by kernel
```

## Go Dependencies

- `filippo.io/age` -- age encryption library (by the age author)
- `golang.org/x/sys/unix` -- keyctl syscalls (no external binary dependency)
- `golang.org/x/term` -- terminal password input (no echo)
- `github.com/spf13/cobra` -- CLI framework

## Project Structure

```
lockbox/
  cmd/
    lockbox/
      main.go                  # entrypoint
  internal/
    cache/
      cache.go                 # SessionCache interface
      keyring_linux.go         # Linux kernel keyring implementation
      keyring_stub.go          # non-Linux stub (build tags)
    store/
      types.go                 # Secret, StoreData types
      store.go                 # Encrypted store operations
    cmd/
      root.go                  # Cobra root command
      init.go                  # init command
      add.go                   # add command
      remove.go                # remove command
      list.go                  # list command
      unlock.go                # unlock command
      lock.go                  # lock command
      get.go                   # get command
      run.go                   # run command
      env.go                   # env command
  go.mod
  go.sum
  README.md
  AGENTS.md
```

## Security Properties

| Concern                      | Mitigation                                                       |
|------------------------------|------------------------------------------------------------------|
| Plaintext on disk            | Never. Store is age-encrypted.                                   |
| Master password storage      | Never stored. Entered interactively, used for decrypt, discarded.|
| Memory exposure              | Best-effort zeroing after loading into kernel keyring.           |
| Other users on same system   | Kernel keyring is per-UID.                                       |
| Other processes same session | Same trust boundary as ssh-agent. `run` mitigates by keeping     |
|                              | secrets out of the parent shell.                                 |
| File permissions             | Store file: 0600. Directory: 0700.                               |
| Shell history                | Secret values never in command arguments -- always prompted.     |
| `/proc/PID/environ`         | With `run`, secrets only in child's environ.                     |

## SessionCache Interface

```go
type SessionCache interface {
    Store(name, value string) error
    Retrieve(name string) (string, error)
    List() ([]string, error)
    Clear() error
    IsUnlocked() bool
}
```

Linux implementation uses keyctl syscalls directly (add_key, keyctl_search,
keyctl_read, keyctl_clear) against the user keyring (`@u`).

## Risks and Considerations

- **tmux/screen**: Tested and confirmed that the session keyring (`@s`) is
  revoked when entering tmux, making cached secrets inaccessible. Switched to
  the user keyring (`@u`) which persists across all sessions for the same UID
  and survives tmux/screen detach-reattach cycles.
- **Container restrictions**: If keyctl syscalls are blocked, fall back to
  decrypt-on-the-fly in `run` (prompts for master password each time).
- **Concurrent access**: Use `flock` on the store file during writes.
- **Name conflict**: Verify `lockbox` isn't taken in Homebrew/apt before
  publishing. The most notable existing "lockbox" is `ankane/lockbox`, a
  Ruby gem for ActiveRecord encryption -- different ecosystem, no conflict.
- **Go GC**: Go's garbage collector means we cannot guarantee secrets are
  zeroed from memory. We minimize the exposure window but this is a known
  limitation of the Go runtime.

## Verification Plan

- [ ] `go build ./...` succeeds
- [ ] `go vet ./...` clean
- [ ] `go test ./...` passes
- [ ] Manual: `lockbox init` -> `add` -> `unlock` -> `run -- env | grep TOKEN` -> `lock`
- [ ] Verify secrets never appear as plaintext files
- [ ] Verify `run` doesn't leak to parent environment
- [ ] Test kernel keyring cleanup on session end

## Future Enhancements (Out of Initial Scope)

- macOS Keychain backend
- Windows Credential Manager backend
- Secret rotation reminders / expiry warnings
- Secret groups / profiles (`--profile work`)
- Shell completion (bash, zsh, fish)
- `git-credential` protocol integration
- Import from 1Password / Bitwarden / pass
