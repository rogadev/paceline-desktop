# paceline-desktop

[paceline](https://github.com/rogadev/paceline) for Claude Desktop: a tray app that paces your weekly usage limit, for Windows, macOS, and Linux.

> **Status: early development.** Nothing is released yet. See the [design](docs/design.md) for the plan and the open questions.

Claude Desktop has no status line, so paceline can't sit under the input box the way it does in Claude Code. paceline-desktop puts the same answer in your system tray instead: **today's budget**, the weekly percentage left divided by the days until your limit resets, counting down as you work.

```
▲ 54% left of today's 28% budget · 84% session · 96% week
```

It comes as two programs:

- **`paceline-desk`**, the tray (menu bar) app. It shows the pace arrow and today's budget at all times, with session and week details in its menu.
- **`paceline-mcp`**, an MCP server for Claude Desktop, so you can ask Claude "how's my budget today?"

## Where the numbers come from

1. **paceline's feed file.** If you also use Claude Code with paceline, paceline can write your latest usage to a local file. No network needed.
2. **Anthropic's usage endpoint.** Otherwise, the tray app reads the usage endpoint using the sign-in Claude Code already stored on your machine. This needs Claude Code installed and signed in, and that sign-in has to have been used recently. The [design](docs/design.md#the-biggest-open-risk-token-expiry) explains why.

paceline-desktop never refreshes or changes your Claude Code sign-in, and the MCP server never touches the network or your credentials.

## License

MIT
