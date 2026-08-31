# Incantations

Small system utilities for people who don't remember the incantation.

Instead of recalling `ps aux | sort -rk 4` or `df -h | grep -v tmpfs`,
type `ram`, `cpu`, or `disk`.

## What you get

| Command | What it shows |
| --- | --- |
| `ram` | Total, used, and available memory, plus the top memory hogs |
| `cpu` | CPU utilization, load average, and the top CPU hogs |
| `disk` | Disk usage for real filesystems, fullest first |

Every command prints plain, human-readable output. No flags to memorize.

## Install

```sh
go install github.com/metruzanca/incantations@latest
```

## Set up your shell

Run this once per shell and drop it in your shell's config file so it's
available in every new terminal.

Bash (`~/.bashrc`):

```sh
eval "$(incantations init bash)"
```

Zsh (`~/.zshrc`):

```sh
eval "$(incantations init zsh)"
```

Fish (`~/.config/fish/config.fish`):

```fish
incantations init fish | source
```

Now `ram`, `cpu`, and `disk` just work. Adding new commands to Incantations
won't ever require changes to your shell config.

## Examples

```
$ ram
Memory
Total         31.2 GiB
Used          16.7 GiB  53.4%
Available     14.5 GiB
Buffers/cache 13.3 GiB

Top processes by memory
RSS        %VMEM   COUNT  COMMAND
7.8 GiB     24.9%  37     brave
6.2 GiB     19.9%  8      opencode
2.1 GiB      6.6%  12     steamwebhelper
```

```
$ cpu
CPU utilization (over 300ms)
User:       8.7%
System:     2.7%
Idle:      88.6%
Load average (1m 5m 15m): 2.32 1.95 1.86

Top processes by CPU
  PID     %CPU    RSS        COMMAND
  533343   40.0%  909.1 MiB  .opencode-wrapp
```

```
$ disk
Disk usage
Filesystem      Type  Size   Used  Avail  Use%    Mounted on
/dev/nvme0n1p2  ext4  1.7T   376G  1.3T      24%  /
/dev/nvme0n1p1  vfat  1022M  240M  783M      24%  /boot
```

## Troubleshooting

- `incantations: command not found` — install the binary, then re-run the
  `eval` line for your shell.
- Output says `not yet supported on ...` — that command isn't implemented on
  your platform yet.

## Adding a command

Mirror an existing one: put a small package under `internal/`, add its `Spec`
to `internal/app`, rebuild. `init` picks it up automatically.