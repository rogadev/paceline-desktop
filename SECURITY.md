# Security policy

## Reporting a vulnerability

Please report security issues privately through [GitHub's private vulnerability reporting](https://github.com/rogadev/paceline-desktop/security/advisories/new), not in a public issue. You'll get a response within a few days.

## Scope

paceline-desktop runs in the background with your privileges and reads the OAuth token Claude Code stores on your machine. The issues that matter most:

- **Credential exposure.** Any way the token, or anything else in Claude Code's credentials file, can be logged, written to disk, sent anywhere other than Anthropic's usage endpoint, or reached through the MCP server.
- **Credential damage.** Any way paceline-desktop can modify, refresh, or invalidate your Claude Code sign-in.
- **Code execution.** Any way that a state file, feed file, or API response can make paceline-desktop run a command.
- **Supply chain.** Anything that could make a release binary differ from what this repository's code builds.

## What the project does

- Only `internal/usage/oauth` may use the network, and only `internal/creds` may start a process (the macOS `security` CLI). The MCP server links neither. Tests enforce these rules.
- Every third-party module must be on a reviewed allowlist, or the tests fail.
- The credentials file is read, never written, and only the access token and its expiry are decoded.
- State and feed files are size-capped and validated before use. A bad file is treated as missing.
- Go source must be ASCII, so bidi overrides and zero-width characters can't hide in the code.
