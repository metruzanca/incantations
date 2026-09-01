# Incantations

Small system utilities for people who don't remember the linux incantations.

Instead of recalling `ps aux | sort -rk 4` or `df -h | grep -v tmpfs`,
type `ram`, `cpu`, or `disk`.

## What you get

| Command | What it shows |
| --- | --- |
| `ram` | RAM and swap usage with a usage bar, plus the top memory hogs |
| `cpu` | The top CPU hogs; add `-t` for the utilization breakdown and load average |
| `disk` | Disk usage per real filesystem with a usage bar; `-a` shows small partitions |

## Install

```sh
go install github.com/metruzanca/incantations@latest
```

New versions are published as git tags, so `go install ...@latest` always
picks up the newest release (or pin one with `@v0.x.y`).

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
 TYPE   USAGE                                                  
 RAM    ███████████░░░░░░░░░  55%  18.5GB/33.5GB (15.0GB Free) 
 SWAP   ████████████░░░░░░░░  61%  5.6GB/9.1GB (3.6GB Free)    

 COMMAND   MEMORY  PROCESSES  % OF MEMORY 
 chrome    1.5 GB          1         4.4% 
 kthreadd  2.1 MB          1         0.0% 
```

```
$ cpu
 COMMAND               PID  % OF CPU  MEMORY 
 web server (worker)  4321     99.9%  1.5 GB 
 kthreadd              555      1.2%  2.1 MB 
```

Percentages are of the whole CPU (all cores combined), not per core. The
utilization breakdown and load average are usually noise, so they're tucked
behind `cpu -t` (also `sys -t`):

```
$ cpu -t
Programs        9.3%
System          2.9%
Idle           87.8%
Load (1m 5m 15m) 2.01 1.83 1.77

 COMMAND            PID  % OF CPU  MEMORY 
 opencode            533343      4.5%  900 MB  
 gpu-screen-recorder  318925      1.7%  448 MB  
 gnome-shell          3024       0.8%  459 MB  
```

```
$ disk
 FILESYSTEM        TYPE  USAGE                                             MOUNTED ON 
 nfs:/data/shared  nfs4  ████████████████░░░░  81%  3.2T/4.0T (800G Free)  /mnt/shared
 /dev/nvme0n1p2    ext4  █████░░░░░░░░░░░░░░░  24%  377G/1.7T (1.3T Free)  /
```

Small filesystems (like a sub-1GB `/boot`) are hidden by default to cut noise.
Show everything with `disk -a` (or `--all`).
