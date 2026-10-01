# paceline-desktop design

This is the working design for paceline-desktop. It builds on the research in the original findings note (October 1, 2026) and records the decisions made since, the facts checked on a real machine, and the questions still open.

## Goal

Give Claude Desktop users what [paceline](https://github.com/rogadev/paceline) gives Claude Code users: an always-visible answer to "how much of my weekly limit can I spend today?"

Claude Desktop has no status line hook. The `statusLine` setting is ignored there ([anthropics/claude-code#41456](https://github.com/anthropics/claude-code/issues/41456)), and the footer ring shows context and plan usage only on click, with no pacing. Other tools work around this with macOS-only menu bar apps or by patching Desktop. Nothing cross-platform offers pacing, which is paceline's differentiator.

## Product shape

Two binaries, for Windows, macOS, and Linux:

| Binary | What it does |
|---|---|
| `paceline-desk` | A tray (menu bar) app. It owns polling, writes the shared state file, and shows the pace state at all times. |
| `paceline-mcp` | A stdio MCP server that Claude Desktop launches. It answers "how's my budget today?" from the state file. |

They're separate binaries because the tray app on Windows must be built with `-H windowsgui` (no console window), and an MCP server needs a working stdin and stdout.

### What the tray shows

Only the parts of paceline that matter to a chat user:

- Today's budget and how much of it is left, with the pace arrow (▲ room to push, ▼ ease off, ● about even).
- Session (five-hour) and week percentages left, and their reset times.
- How fresh the reading is, and where it came from.

Branch, project folder, context, cache, and duration segments don't apply to Desktop and are left out.

Platform limits shape the display:

- **Windows** shows only an icon and a tooltip in the tray, never text. The icon itself must carry the state: the arrow and a fill level, colored by headroom. The tooltip is capped at 127 characters, so it's the one-line summary, for example `▲ 54% left of today's 28% budget · 84% session · 96% week`.
- **macOS** can also show a short title next to the icon (`SetTitle`), for example `▲ 54%`.
- **Linux** with stock GNOME hides tray icons unless the AppIndicator extension is installed. KDE, Xfce, and most others show them. The README must say this.

The menu repeats the full line, the reset times, the data source and its age, "Refresh now", "Start at login", and "Quit".

### MCP tools

Read-only, both answered from the state file:

- `get_usage`: session and week percentages, reset times, the source, and the reading's age.
- `get_today_budget`: today's budget, spent, left, the pace direction, and the same freshness fields.

**Decision: `paceline-mcp` never touches the network or the OAuth token.** The findings proposed that the MCP server fetch once when the state file is stale. Instead, it reports the staleness ("the tray app isn't running; this reading is 3 hours old") and suggests starting the tray app. This keeps the binary that Claude talks to free of network code and credentials, which the import policy test enforces (`net/http` is allowed only in `internal/usage/oauth`). It also removes the need for the two processes to coordinate polling.

## Data sources

Every source returns the same `usage.Usage` value (see `internal/usage`). `usage.First` tries them in order and takes the first valid reading.

### 1. The feed file (preferred)

paceline gains an opt-in setting that writes Claude Code's `rate_limits` payload to `$CLAUDE_CONFIG_DIR/paceline-feed.json` (default `~/.claude/paceline-feed.json`) on each render. It's a local file write, so paceline stays network-free. Proposed format:

```json
{
  "version": 1,
  "fiveHour": { "usedPct": 21, "resetsAt": 1790283000 },
  "sevenDay": { "usedPct": 46, "resetsAt": 1790838000 },
  "writtenAt": 1790277013
}
```

The feed is only as fresh as the last Claude Code render. paceline-desktop uses it when it's newer than the last OAuth reading and less than 10 minutes old.

### 2. The OAuth usage endpoint (fallback)

`GET https://api.anthropic.com/api/oauth/usage` with Claude Code's stored OAuth access token. The endpoint is **undocumented** and could change or disappear. Poll every 5 minutes with jitter, back off exponentially to 30 minutes on 429 or 5xx responses, and allow "Refresh now" at most once a minute.

Where the token lives:

| OS | Location | Status |
|---|---|---|
| Windows | `~/.claude/.credentials.json`, key `claudeAiOauth` | **Verified** October 1, 2026 |
| Linux | `~/.claude/.credentials.json`, key `claudeAiOauth` | From the findings, not yet verified |
| macOS | Keychain item `Claude Code-credentials` | From the findings, not yet verified |

All three honor `CLAUDE_CONFIG_DIR` for the file path.

What the check on Windows found:

- `claudeAiOauth` holds `accessToken`, `refreshToken`, `expiresAt` and `refreshTokenExpiresAt` (Unix milliseconds), `scopes`, `subscriptionType`, and `rateLimitTier`.
- **The same file also holds every MCP server's OAuth tokens and client secrets** under `mcpOAuth`. `internal/creds` must decode only `claudeAiOauth.accessToken` and `expiresAt` into a narrow struct, never keep the rest in memory, never log any of it, and never write to the file.
- The access token expires within hours. On the test machine it had about 4 hours left shortly after Claude Code refreshed it. The refresh token had about 6 days left.

## The biggest open risk: token expiry

The findings flagged that users who never signed in to Claude Code have no token. The check above makes it sharper: **even users who signed in once have a usable token only for a few hours after Claude Code last refreshed it.** For a Desktop-only user, the OAuth fallback stops working the same day.

paceline-desktop could refresh the token itself, but refresh tokens usually rotate. Using one would likely invalidate the copy Claude Code holds and sign the user out of Claude Code, or race Claude Code writing the same file. Writing to a file that also holds other services' secrets is a risk paceline-desktop shouldn't take.

**Decision for v1: read-only, never refresh.** When the token is missing or expired, the tray shows a clear "Open Claude Code once to refresh your sign-in" state instead of numbers, and keeps the last reading with its age.

Options to evaluate before a non-technical rollout:

1. Accept the limit and target people who use Claude Code at least daily (the feed file covers them anyway).
2. Find a usage source that Claude Desktop itself exposes. Desktop's own session is stored by Electron and isn't usable as-is. Needs a spike.
3. Ask Anthropic for a supported usage API or a Desktop status hook (#41456).
4. Run paceline-desktop's own sign-in. This means impersonating Claude Code's OAuth client, which is fragile and likely against the terms. Not recommended.

Before any public release, also confirm that reading Claude Code's stored token from a separate local app is acceptable under Anthropic's terms.

## Sharing the pace math with paceline

paceline's budget math lives in `internal/pace`, which another module can't import. Both tools must compute the same budget, so the plan is to move it to a public package in paceline (for example `github.com/rogadev/paceline/pace`, with the `timefmt` helpers it needs) and import it here. Copying the code is the fallback, with a parity test, if exporting it is undesirable.

Two details matter once both tools compute a budget:

- **Share the day anchor.** paceline anchors today's budget in `$CLAUDE_CONFIG_DIR/paceline-day.json` at the first render of the day. paceline-desktop should read and write the same file through the same exported functions, so the tray and the status line always show the same budget. paceline already writes it atomically, so concurrent writers are safe.
- **Normalize `resetsAt` to whole Unix seconds in every source.** The snapshot treats a changed `resetsAt` as a new week and re-anchors the day. The OAuth endpoint returns an ISO 8601 string that may carry fractional seconds, while the status payload sends whole seconds. If they differ by even a fraction, alternating between the feed and OAuth would reset today's budget on every switch. A test must cover this.

## Package layout

```
paceline-desktop/
  cmd/paceline-desk/     tray app: polling loop, single-instance lock
  cmd/paceline-mcp/      stdio MCP server: reads state only
  internal/usage/        Usage, Window, Source, First           (exists)
  internal/usage/feed/   reads paceline-feed.json
  internal/usage/oauth/  polls the usage endpoint; the only package allowed to use net/http
  internal/creds/        per-OS token lookup (build-tagged files); the only package allowed os/exec (macOS `security`)
  internal/state/        shared state file, atomic writes       (exists)
  internal/tray/         icon rendering, menu, autostart
  internal/mcpserver/    MCP tool definitions
  internal/policy/       import and dependency rules            (exists)
```

## Tech stack

| Concern | Choice |
|---|---|
| Language | Go 1.26, matching paceline |
| HTTP | `net/http`, confined to `internal/usage/oauth` |
| State and IPC | JSON file with temp-file-and-rename writes; the tray polls the file's modification time rather than adding `fsnotify` |
| Single instance | `gofrs/flock` on a lock file next to the state file |
| Tray | `fyne.io/systray`: Win32 and D-Bus StatusNotifierItem without cgo, Cocoa with cgo |
| Icons | stdlib `image`, drawn at runtime and encoded as PNG or ICO |
| Autostart | `HKCU\...\Run` on Windows, a LaunchAgent plist on macOS, an XDG `.desktop` file on Linux |
| Credentials | file read on Windows and Linux; on macOS, the `security` CLI or `zalando/go-keyring` |
| MCP | `modelcontextprotocol/go-sdk` over stdio |
| MCP distribution | an `.mcpb` Desktop extension for one-click install |
| Notifications (later) | `gen2brain/beeep` |

Every third-party module must be added to `allowedModules` in `internal/policy`, with its reason, or the tests fail.

## Build and release

- **macOS needs cgo for the tray,** so darwin builds run on a macOS runner and are merged into a universal binary. Windows and Linux build without cgo.
- **Sign everything** before a non-technical rollout: Apple Developer ID with notarization, and Authenticode or Azure Trusted Signing on Windows. Unsigned builds hit Gatekeeper and SmartScreen warnings.
- Linux ships a tarball plus `.deb` and `.rpm` through GoReleaser's nFPM.
- Versioning follows paceline: Conventional Commits, semantic-release on merge to `main`, GoReleaser for the binaries, and signed build provenance attestations. This is wired up in milestone 6, once there's a binary to release.

## Milestones

1. **In paceline:** export the pace math as a public package, add the opt-in feed file, and add a CI guard that `go list -deps ./cmd/paceline` never includes `net` or `os/exec`.
2. **Foundation** (started): `usage` and `state` packages with tests, and the import policy. Next: the feed source.
3. **Tray MVP on Windows:** icon and menu from the state file, the single-instance lock, and the feed source only.
4. **OAuth fallback:** `creds` per OS, the expiry-aware "open Claude Code" state, and backoff. Verify the Linux and macOS token locations.
5. **MCP:** the `paceline-mcp` binary and `.mcpb` packaging.
6. **Release:** CI matrix with a macOS cgo build, signing, autostart, and cross-platform polish.

## Open questions

- Is "use Claude Code at least once a day" an acceptable requirement for the first users? (See token expiry.)
- Does Anthropic's usage endpoint report the same `seven_day` percentage as Claude Code's status payload? Compare the two side by side during milestone 4.
- Does Claude Desktop expose any usage data locally that a companion app could read?
- Is reading Claude Code's token from another local app acceptable under Anthropic's terms?

## Sources

- [paceline](https://github.com/rogadev/paceline)
- [anthropics/claude-code#41456: Add status bar to Desktop App](https://github.com/anthropics/claude-code/issues/41456)
- [anthropics/claude-code#22221: Expose plan usage limits for status line](https://github.com/anthropics/claude-code/issues/22221)
- [Claude Code usage status line guide (OAuth endpoint, credential locations)](https://gist.github.com/jtbr/4f99671d1cee06b44106456958caba8b)
- [claude-code-usage-bar (desktop HUD prior art)](https://github.com/leeguooooo/claude-code-usage-bar)
