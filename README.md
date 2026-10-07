# paceline-tray

[paceline](https://github.com/rogadev/paceline) in your system tray: a companion app that paces your weekly Claude Code usage limit, for Windows, macOS, and Linux.

> **Status: early development.** Nothing is released yet. See the [design](docs/design.md) for the plan and the open questions.

paceline's status line shows **today's budget** (the weekly percentage left divided by the days until your limit resets) under Claude Code's input box. You only see it while a Claude Code session is open and in front of you. paceline-tray keeps the same answer in your system tray, so you can check it before you start a session, while the terminal is hidden, or across several sessions at once.

```
▲ 54% left of today's 28% budget · 84% session · 96% week
```

The tray (menu bar) app shows the pace arrow and today's budget at all times, with session and week details in its menu.

To let Claude check the budget itself, for example before it starts a long task, or so you can ask "how's my budget today?", use the budget tools in paceline's own MCP server. They don't need paceline-tray.

## Where the numbers come from

1. **paceline's feed file.** paceline writes your latest usage to a local file each time the status line renders. No network needed.
2. **Anthropic's usage endpoint.** When the feed is stale, for example first thing in the morning before you open Claude Code, the tray app reads the usage endpoint using the sign-in Claude Code already stored on your machine. That sign-in has to have been used recently. The [design](docs/design.md#token-expiry) explains why.

paceline-tray never refreshes or changes your Claude Code sign-in.

## License

MIT
