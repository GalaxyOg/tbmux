---
name: tbmux
description: Manage TensorBoard run aggregation with tbmux on Linux hosts. Use when requests mention tbmux, TensorBoard run discovery/selection, `tbmux sync/list/select/start/stop/status/open`, or Tailscale serve setup for TensorBoard access.
---

# tbmux Skill

## Overview

Use this skill to operate `tbmux` end to end: discover runs, maintain the selected run set, apply symlink aggregation, and control TensorBoard/Tailscale exposure.

## Workflow

1. Verify environment first.
`tbmux version`
`tbmux doctor`
`tbmux config path`

2. Initialize config if needed.
`tbmux init`
Then edit `~/.config/tbmux/config.toml` and set `[[watched_roots]]`.

3. Refresh run discovery and inspect candidates.
`tbmux sync`
`tbmux list --running`
`tbmux list --hours 24`
`tbmux list --under /path/to/project --match keyword`

4. Update selected set and apply aggregation.
`tbmux select clear`
`tbmux select by-filter --running --set`
`tbmux select by-filter --match keyword`
`tbmux select apply`

5. Control serving status.
`tbmux start`
`tbmux status`
`tbmux open`
`tbmux tailscale status`
`tbmux tailscale serve --dry-run`
`tbmux tailscale serve`

## Command Guardrails

- Prefer read-only checks before mutations (`doctor`, `status`, `list`, `selected list`).
- Run `tbmux sync` before changing selected runs.
- Use `--json` on `doctor/sync/list/status/tailscale status|serve` when output will be parsed.
- If `tbmux start` reports TensorBoard not found, set `[tensorboard].binary` to an absolute path.
- Keep `tbmux select ...` and `tbmux select apply` together; draft changes are not active until apply.

## Quick Commands

```bash
# first-time setup
tbmux init
tbmux config example

# daily flow
tbmux sync
tbmux list --running --hours 24
tbmux select by-filter --running --set
tbmux select apply
tbmux start
tbmux status

# troubleshooting
tbmux doctor --json
tbmux tailscale status --json
```

## References

Read `references/commands.md` for command matrix, filters, and troubleshooting.
