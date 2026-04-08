# lockbox

Secure secret management for headless Linux systems.

`lockbox` provides an **ssh-agent-style experience** for API tokens, PATs, and
other secrets. Secrets are persisted in an
[age](https://github.com/FiloSottile/age)-encrypted store and cached in the
Linux kernel keyring during a session. Unlock once with a master password, then
use your secrets without re-entering it.

## Why lockbox?

If you work on headless Linux machines over SSH and need to authenticate with
services like GitHub, Jira, or Confluence, you've probably hit these problems:

| Existing tool | Problem on headless Linux |
|---|---|
| GNOME Keyring / KWallet | Falls back to **plaintext** without a GUI session |
| `keepassxc-cli` | Master password required on **every access** |
| HashiCorp Vault | Requires running a **server** |
| 1Password CLI / Bitwarden CLI | Requires a **third-party subscription** |
| `pass` + `gpg-agent` | Works, but **GPG setup is painful** |
| Kernel keyring (`keyctl`) alone | No persistence -- secrets **lost on logout** |

`lockbox` combines the best parts: age encryption for at-rest storage +
kernel keyring for session caching. No daemon, no GUI, no subscriptions.

## How it works

```
  Encrypted store on disk              Session cache in kernel memory
  ~/.config/lockbox/store.age    -->   Linux kernel keyring (@s)
  (age + scrypt)                       (auto-destroyed on logout)
                    \                  /
                     lockbox unlock
                     (enter master password once)
```

**Secrets never exist as plaintext on the filesystem.** The store is
age-encrypted at rest. During a session, secrets live in kernel memory
(the same mechanism that backs `ssh-agent` keys).

## Quick start

### Install

```bash
go install github.com/tenortim/lockbox/cmd/lockbox@latest
```

### First-time setup

```bash
# Create an encrypted store
lockbox init
# Enter master password: ****
# Confirm master password: ****
# Store created at ~/.config/lockbox/store.age

# Add secrets (with optional expiry)
lockbox add github_pat --env GITHUB_TOKEN --desc "GitHub PAT" --expires 90d
# Enter secret value: ****
# Enter master password: ****
# Secret 'github_pat' added (env: GITHUB_TOKEN), expires 2026-07-07

lockbox add jira_token --env JIRA_API_TOKEN --desc "Jira Cloud" --expires 2026-12-31
# ...
```

### Daily usage

```bash
# Start of session: unlock once
lockbox unlock
# Enter master password: ****
# 2 secret(s) loaded into session cache.

# Run commands with secrets injected (most secure)
lockbox run -- gh pr list
lockbox run -- curl -H "Authorization: Bearer $(lockbox get jira_token)" ...

# Or inject only specific secrets
lockbox run --secrets github_pat -- gh pr list

# End of session
lockbox lock
# Or just disconnect -- the kernel destroys the session keyring automatically
```

### Alternative: export to shell

If you prefer secrets in your shell environment (less secure but convenient
for interactive use):

```bash
eval $(lockbox env)
# GITHUB_TOKEN and JIRA_API_TOKEN are now exported

# Or export only specific secrets
eval $(lockbox env --secrets github_pat)
```

## Commands

| Command | Description |
|---|---|
| `lockbox init` | Create a new encrypted store |
| `lockbox add <name> --env VAR` | Add a secret with an env var mapping |
| `lockbox remove <name>` | Remove a secret |
| `lockbox list` | List secret names and env var mappings (no values) |
| `lockbox unlock` | Decrypt store and load secrets into session cache |
| `lockbox lock` | Clear all secrets from session cache |
| `lockbox status` | Show lock/unlock state and cached secrets |
| `lockbox get <name>` | Retrieve a single secret value |
| `lockbox run [--secrets ...] -- cmd` | Run a command with secrets as env vars |
| `lockbox env [--secrets ...]` | Print `export` statements for `eval` |
| `lockbox completion <shell>` | Generate shell completion scripts |

### Global flags

| Flag | Default | Description |
|---|---|---|
| `--store PATH` | `~/.config/lockbox/store.age` | Path to the encrypted store file |

Override the default store location with `--store` or the `LOCKBOX_STORE`
environment variable.

## Security model

### What lockbox protects against

- **Secrets on disk**: The store is encrypted with
  [age](https://github.com/FiloSottile/age) using passphrase-based scrypt
  encryption. There is no point at which secrets exist as plaintext files.
- **Shell history exposure**: Secret values are always entered via password
  prompts (no echo) or read from the session cache. They never appear as
  command-line arguments.
- **Environment leakage** (with `run`): `lockbox run` injects secrets only
  into the child process's environment via `execve`. The parent shell's
  environment is never modified, and the secrets are not visible in
  `/proc/<parent-pid>/environ`.
- **Post-session access**: The kernel session keyring is destroyed when the
  login session ends. No cleanup needed.
- **Other users**: The kernel keyring is per-UID. Other users on the same
  system cannot access your cached secrets.

### Trust boundaries

- **Same-session processes**: Any process running as your user in the same
  session can read the session keyring. This is the same trust boundary as
  `ssh-agent`. Use `lockbox run` to minimize the window -- secrets are only
  in the child process's environment.
- **`lockbox env` / `eval`**: Exporting secrets to the shell environment means
  every subsequent command inherits them. Use `lockbox run` when possible.
- **Go garbage collector**: Go's GC means we cannot guarantee secrets are
  zeroed from process memory. We minimize the exposure window but cannot
  eliminate it entirely. This is a known limitation shared by most Go-based
  secret management tools.

### File permissions

- Store directory (`~/.config/lockbox/`): `0700`
- Store file (`store.age`): `0600`

These are enforced on creation.

## `lockbox run` vs `lockbox env`

| | `lockbox run` | `lockbox env` |
|---|---|---|
| **How** | `lockbox run -- gh pr list` | `eval $(lockbox env)` |
| **Scope** | Child process only | Current shell + all children |
| **`/proc/PID/environ`** | Only in child | In shell process |
| **Cleanup** | Automatic (process exits) | Manual (`unset VAR`) or new shell |
| **Convenience** | Must prefix every command | Set once, use freely |

**Recommendation**: Use `lockbox run` for scripts and automation. Use
`lockbox env` for interactive sessions where convenience matters.

## Expiry tracking

PATs and API tokens typically have a limited lifetime. lockbox can track
expiry dates so you know when tokens need to be rotated -- before you get
a mysterious 401.

### Setting expiry on add

```bash
# Relative duration from now
lockbox add github_pat --env GITHUB_TOKEN --expires 90d     # 90 days
lockbox add jira_token --env JIRA_API_TOKEN --expires 6m    # 6 months
lockbox add ci_token --env CI_TOKEN --expires 1y             # 1 year

# Absolute date
lockbox add deploy_key --env DEPLOY_KEY --expires 2026-12-31
```

Supported duration units: `d` (days), `w` (weeks), `m` (months), `y` (years).

### Where warnings appear

**`lockbox unlock`** warns about expired or soon-to-expire secrets on load:

```
3 secret(s) loaded into session cache.
  WARNING: 'github_pat' (GITHUB_TOKEN) has EXPIRED
  WARNING: 'jira_token' (JIRA_API_TOKEN) expires in 5 days
```

**`lockbox list`** shows an EXPIRES column:

```
NAME          ENV VAR          EXPIRES          DESCRIPTION
github_pat    GITHUB_TOKEN     EXPIRED          GitHub PAT
jira_token    JIRA_API_TOKEN   expires in 5d    Jira Cloud
deploy_key    DEPLOY_KEY       2026-12-31       Production deploy
ci_token      CI_TOKEN         -                CI (no expiry set)
```

**`lockbox status`** flags problematic secrets when unlocked:

```
  github_pat -> GITHUB_TOKEN  [EXPIRED]
  jira_token -> JIRA_API_TOKEN  [expires in 5 days]
  deploy_key -> DEPLOY_KEY  (expires 2026-12-31)
```

Secrets with no expiry set are never flagged -- the `--expires` flag is
entirely optional.

## tmux / screen

The Linux kernel session keyring is tied to the login session. When you
detach from a tmux or screen session and reconnect, you get a new session
keyring -- your cached secrets will be gone and you'll need to `lockbox unlock`
again.

If this is a frequent workflow, you can use the user keyring instead, which
persists as long as you're logged in somewhere on the system. (This is not
yet exposed as a CLI flag but is supported in the code.)

## Shell integration

### Tab completion

lockbox provides tab completion for commands, flags, and secret names
(from the session cache) for bash, zsh, fish, and PowerShell.

**Bash:**
```bash
# Add to ~/.bashrc
eval "$(lockbox completion bash)"
```

**Zsh:**
```bash
# Add to ~/.zshrc
eval "$(lockbox completion zsh)"
```

**Fish:**
```bash
# Add to ~/.config/fish/config.fish
lockbox completion fish | source
```

With completion enabled, you can tab-complete secret names:

```bash
$ lockbox get gi<TAB>        ->  lockbox get github_pat
$ lockbox run --secrets j<TAB>  ->  lockbox run --secrets jira_token
```

### Prompt integration

Show lock/unlock status in your shell prompt using `lockbox status --short`:

```
# Locked:   "locked"
# Unlocked: "unlocked 3"
```

This reads directly from the kernel keyring (a syscall, no disk I/O) so it
adds negligible latency to your prompt.

**Bash:**
```bash
# Add to ~/.bashrc
__lockbox_ps1() {
    local status
    status=$(lockbox status --short 2>/dev/null)
    case "$status" in
        unlocked*) echo "[lb: $status]" ;;
        locked)    echo "[lb: locked]" ;;
    esac
}
PS1="\u@\h \w \$(__lockbox_ps1) \$ "
```

**Zsh:**
```bash
# Add to ~/.zshrc
__lockbox_ps1() {
    local status
    status=$(lockbox status --short 2>/dev/null)
    case "$status" in
        unlocked*) echo "[lb: $status]" ;;
        locked)    echo "[lb: locked]" ;;
    esac
}
setopt PROMPT_SUBST
RPROMPT='$(__lockbox_ps1)'
```

This gives you a prompt like:

```
user@host ~/project [lb: unlocked 3] $
```

### Auto-unlock on login

Prompt to unlock at the start of each SSH session:

```bash
# Add to ~/.bashrc
if command -v lockbox &>/dev/null \
    && [ -f "$(lockbox status 2>/dev/null | awk '/^Store:/{print $2}')" ] \
    && lockbox status --short 2>/dev/null | grep -q locked; then
    echo "lockbox store found but locked. Unlock now? (Ctrl+C to skip)"
    lockbox unlock
fi
```

### Convenience alias

A shorthand for `lockbox run`:

```bash
# Add to ~/.bashrc or ~/.zshrc
lbr() { lockbox run -- "$@"; }

# Usage:
lbr gh pr list
lbr curl -H "Authorization: Bearer $JIRA_API_TOKEN" ...
```

## Building from source

```bash
git clone https://github.com/tenortim/lockbox.git
cd lockbox
go build -o lockbox ./cmd/lockbox/
```

### Requirements

- Go 1.21 or later
- Linux (kernel keyring support)

### Running tests

```bash
go test ./...
```

The cache tests exercise the real kernel keyring and will be skipped
automatically on non-Linux systems or environments where the keyring is
unavailable.

## How it compares

| Feature | lockbox | `pass`+GPG | 1Password CLI | `keyctl` alone |
|---|---|---|---|---|
| Encrypted at rest | age (scrypt) | GPG | Cloud | N/A |
| Session caching | Kernel keyring | gpg-agent | `op` session | Kernel keyring |
| Unlock model | Master password once | GPG passphrase once | Account login | N/A |
| Persistence | Encrypted file | GPG files + git | Cloud | None |
| Headless / SSH | Yes | Yes | Yes | Yes |
| External dependency | None | GPG + key mgmt | Subscription | None |
| Env var injection | `run` / `env` | Manual | `op run` | Manual |
| Setup complexity | `lockbox init` | GPG keygen + init | Account + install | N/A |

## License

TBD
