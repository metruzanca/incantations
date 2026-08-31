# Incantations

Small system utilities for people who don't remember the linux incantations.

Instead of recalling `ps aux | sort -rk 4` or `df -h | grep -v tmpfs`,
type `ram`, `cpu`, or `disk`.

## What you get

| Command | What it shows |
| --- | --- |
| `ram` | Total, used, and available memory, plus the top memory hogs |
| `cpu` | CPU utilization, load average, and the top CPU hogs |
| `disk` | Disk usage for real filesystems with a usage bar; `-a` shows small partitions |

## Install

```sh
go install github.com/metruzanca/incantations@latest
```

## Set up your shell

Run `incantations init` once, follow its advice, and you're done. It detects
your shell and prints the exact line to add to your config file.

That line defines a shell function for each utility, so you can type a plain
`ram`, `cpu`, or `disk` instead of `incantations ram`. Everything stays one
small binary behind a few functions rather than a cluttered pile of separately
installed utilities, and adding or upgrading commands never touches your
shell config again.

## Examples

```
$ ram
██████████░░░░░░░░░░  51% used
Total         33.5 GB
Used          16.8 GB
Available     16.7 GB
Cache         13.2 GB

 COMMAND         MEMORY  PROCESSES  % OF MEMORY 
  brave           9.0 GB         37        27.1%  
 opencode        6.6 GB          8        20.0%   
 steamwebhelper  1.9 GB         12         5.7%   
```

```
$ cpu
Programs        9.3%
System          2.9%
Idle           87.8%
Load (1m 5m 15m) 2.01 1.83 1.77

 COMMAND            PID  % OF CPU  MEMORY 
  opencode            533343      4.5%  900 MB  
 gpu-screen-recorder  318925      1.7%  448 MB  
 gnome-shell          3024       0.8%  459 MB  
```

Percentages are of the whole CPU (all cores combined), not per core.

Small filesystems (like a sub-1GB `/boot`) are hidden by default to cut noise.
Show everything with `disk -a` (or `--all`).
