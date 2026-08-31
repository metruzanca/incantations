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
RAM
Total         33.5 GB
Used          17.2 GB   51%
Available     16.3 GB
Cache         13.1 GB

Top processes by memory
 COMMAND         MEMORY  PROCESSES  % OF MEMORY 
  brave           9.0 GB         37        27.1%  
 opencode        6.6 GB          8        20.0%   
 steamwebhelper  1.9 GB         12         5.7%   
```

```
$ cpu
CPU usage (last 300ms)
Programs        9.3%
System          2.9%
Idle           87.8%
Load (1m 5m 15m) 2.01 1.83 1.77

Top processes by CPU
 COMMAND            PID    CPU  MEMORY 
  opencode            533343  45.0%  900 MB  
 gpu-screen-recorder  318925  20.0%  448 MB  
 gnome-shell          3024    10.0%  459 MB  
```

```
$ disk
Disk usage
 FILESYSTEM      TYPE  SIZE  USED  AVAILABLE  USED %  MOUNTED ON 
  /dev/nvme0n1p2  ext4  1.7T  376G       1.3T     24%  /           
 /dev/nvme0n1p1  vfat  1022M  240M       783M     24%  /boot        
```

## Troubleshooting

- `incantations: command not found` — install the binary, then re-run the
  `eval` line for your shell.
- Output says `not yet supported on ...` — that command isn't implemented on
  your platform yet.

## Adding a command

Mirror an existing one: put a small package under `internal/`, add its `Spec`
to `internal/app`, rebuild. `init` picks it up automatically.