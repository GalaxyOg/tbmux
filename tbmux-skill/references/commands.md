# tbmux Command Reference

## Primary Commands

```bash
tbmux init [--force] [--json]
tbmux sync [--apply] [--json]
tbmux list [--today|--hours N|--days N|--running|--not-running|--under PATH|--match Q] [--json]
tbmux selected list [--json]
tbmux select clear
tbmux select add <id|name>...
tbmux select remove <id|name>...
tbmux select by-filter [--today|--hours N|--days N|--running|--not-running|--under PATH|--match Q] [--set|--remove]
tbmux select apply
tbmux start [--no-sync]
tbmux stop
tbmux restart
tbmux status [--json]
tbmux open
tbmux doctor [--json]
tbmux tailscale status [--json]
tbmux tailscale serve [--dry-run] [--json]
tbmux config path
tbmux config example
tbmux version
tbmux tui
```

## Typical Operation Sequences

```bash
# bootstrap
tbmux init
tbmux doctor

# refresh + select + apply + serve
tbmux sync
tbmux list --running --hours 24
tbmux select by-filter --running --set
tbmux select apply
tbmux start
tbmux status
```

```bash
# safely rotate selected set by keyword
tbmux sync
tbmux select clear
tbmux select by-filter --match exp_keyword
tbmux select apply
tbmux restart
```

## Config Notes

- Config path: `~/.config/tbmux/config.toml`
- If TensorBoard cannot be found, set:
`[tensorboard].binary = "/absolute/path/to/tensorboard"`
- Default log/pid/state paths live under:
`~/.local/state/tbmux` and `~/.local/share/tbmux`

## Troubleshooting

- `tbmux start` fails with missing tensorboard:
Set `[tensorboard].binary` explicitly and rerun `tbmux doctor`.
- No runs discovered:
Check `[[watched_roots]]` paths, permissions, and `exclude_patterns`, then run `tbmux sync`.
- Tailscale URL missing:
Run `tbmux tailscale status`, then `tbmux tailscale serve --dry-run` and `tbmux tailscale serve`.
