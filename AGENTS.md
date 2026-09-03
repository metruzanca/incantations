# Incantations — for agents

Notes for AI agents and human maintainers doing work on this repo. Humans
should prefer `README.md` for usage; this file covers the how-it-works and
how-to-work-here details.

## Commands

```sh
go build ./...        # must pass before every commit
go vet ./...          # run before every commit
go test ./...         # full suite
go test ./... -count=1  # bypass cache if results look stale
go test ./internal/<pkg>/ -run <Test> -update   # regenerate golden files
```

Commit rule: commit at meaningful points, only when `go build ./...`,
`go vet ./...`, and the full test suite pass. This is a CLI; keep every commit
greenscreen.

Releases: pushing a `vX.Y.Z` tag runs GoReleaser (`.github/workflows/release.yml`
+ `.goreleaser.yaml`), which uploads cross-platform binaries and makes the tag
servable to `go install github.com/metruzanca/incantations@latest`.
Check config drift with `goreleaser check`; snapshot builds with
`goreleaser build --snapshot`.

## Layout

- `main.go` — thin entrypoint at the module root (so plain
  `go install github.com/metruzanca/incantations@latest` works). Logging init +
  `app.Run`.
- `internal/app` — dispatch. Owns the `command.Registry`, help/version/errors,
  exit codes (0 ok, 1 runtime error, 2 usage error). `New` is variadic so
  tests inject synthetic entries. `Version` is overridable via
  `-ldflags "-X github.com/metruzanca/incantations/internal/app.Version=vX"`.
- `internal/command` — tiny registry (`Entry{Name, Summary, Meta, Run}`).
  The single source of truth for dispatch and for `init` shell generation.
- `internal/shell` — `Generate(shell, commands)` is pure and deterministic.
  bash/zsh share one template; fish differs (`$argv`). Golden files live in
  `testdata/`; regenerate with `-update`. `init` accepts normalized names
  (`/bin/zsh`, `zsh`, `ZSH` → `zsh`, via `IsShell`), optionally followed by a
  command list (`init bash ram cpu` — shell autodetected from `$SHELL`
  otherwise, via `DetectShell`); its no-arg and `--help` screens detect the
  caller's shell and suggest a copy-paste setup line (`SetupCommand`).
  `app.wrapNames` is the default command list and drops `battery` when
  `battery.HasBattery()` is false, so desktops get no useless wrapper.
- `internal/logutil`, `internal/units`, `internal/ui` — shared plumbing. `units`
  uses decimal GB/MB (laymen units); `ui` renders charmbracelet tables and
  progress bars and is always plain in tests (goldens stay escape-free). Enable
  `ui.Styled` only from main when stdout is a terminal.

## How a command works

Each utility is a self-contained package exposing:

1. pure parsing: `io.Reader` in, typed struct out (no build tags). Tested
   against `testdata/` fixtures, no host dependency.
2. pure rendering: `Render(*Report) string`. Tested against golden files.
3. `Sample() (*Report, error)` — platform-specific acquisition, defined ONLY
   in build-tagged files:
   - `*_linux.go` reads `/proc`.
   - `*_unsupported.go` (`//go:build !linux`) returns
     `fmt.Errorf("not yet supported on %s", runtime.GOOS)`. Windows disk has
     its own `disk_windows.go` stub.
   - Do NOT forward-declare `func Sample()` in the untagged file — Go treats a
     body-less declaration as assembly and it collides with the real one.
4. `Spec()` returns a `command.Entry` wired to `Sample` + `Render`, plus an
   optional `Help` string shown by `incantations <name> --help` (the app
   falls back to a generic usage line for entries without one).

To add a command: create `internal/<name>`, register its `Spec()` in
`app.New`, add fixture/golden tests. No shell-config work: `init` regenerates
functions from the registry automatically.

`sys` is a composed command: it calls `ram.Sample()`, `cpu.Sample()`, and
`disk.Sample()` and concatenates their rendered reports. Because every
subsystem has a `Sample` defined for every platform, `sys` needs no build-tag
files of its own.

## Platform contract

- Linux reads `/proc` directly (no cgo, no external deps): `meminfo`,
  `/proc/<pid>/status`, `/proc/stat`, `/proc/<pid>/stat`, `/proc/loadavg`.
- Process display names come from the first token of `/proc/<pid>/cmdline`
  argv[0] (falling back to `comm`, which the kernel truncates to 15 chars).
  `ram` groups same-named processes (summed RSS + count); `cpu` lists per PID.
  The `/proc/self/exe` argv[0] linker trick is treated as no name.
- `disk` shells out to `df -hT` on any non-Windows platform; `parseDf` is the
  pure, testable core. Virtual filesystems are filtered via `hiddenTypes`
  (tmpfs/overlay/proc/etc.), and filesystems smaller than 1 GiB are hidden
  unless the `-a`/`--all` flag is passed (see `sizeValue`). Renders one
  progress bar per row.
- `space` shells out to `du -x -B1 --max-depth=1` and parses with `parseDu`;
  GNU du prints the scanned root *last*, so `parseDu` matches the root line by
  path, not position. `Render` shows each dir's share of the root total, with
  the same 1 GiB default cutoff as `disk`. No PATH defaults to `$HOME`
  (`os.UserHomeDir`), which is fast compared to scanning `/`. Walking every
  file makes it slow — the help text says so.
- `battery` reads sysfs power-supply `uevent` files directly
  (`$POWER_ROOT/<dev>/uevent`); `parseUevent` handles both ENERGY_* (µWh) and
  CHARGE_* (µAh) batteries by scaling charge with the voltage. A machine with
  no battery reports `Found=false` (-> "No battery found."), not an error, so
  desktop users see a clean one-liner. `powerRoot` is a package var so tests
  inject a fake `/sys/class/power_supply`. Non-Linux returns the unsupported
  stub.
- `ports` shells out to `ss -ltulpn` (Linux-only) like `disk` shells to `df`;
  `parseSs` is the pure core, and `parseUsers` extracts name+PID from the
  `users:(("name",pid=...))` column. `parseServices` reads `/etc/services`
  for the `0.0.0.0:22 (ssh)` labels. Without root, other users' sockets show
  no PID and group under `-`. `Render` shows TCP over IPv4 by default, grouped
  per process (`groupByPID`, unowned last); `--udp` and `--ipv6` unhide the
  rest (IPv6 is detected by the `[` address prefix, since ss labels the netid
  `tcp`/`udp` for both families). An optional numeric `PORT` filters the
  listing. `--stop PORT`/`--kill PORT` send SIGTERM/SIGKILL to the owned PIDs
  on that port via `signalPort`; the signal itself is `signalProcess`, a
  package var (real `syscall.Kill` on Linux, swappable in tests), with
  `termSignal`/`killSignal` defined per build tag.
- `net` samples `/proc/net/dev` twice (~1s apart) for live RX/TX rates
  (`parseDev` + `computeRates` are the pure, fixture-tested bits) and resolves
  IPs from the stdlib `net` package. Interface counters are cumulative, so a
  counter that goes backwards (reset) reads 0 for that window. IPv6 addresses
  are hidden unless `--ipv6` is passed (`visibleAddrs`), loopback unless `-a`.
- Non-Linux `ram`/`cpu` use `*_unsupported.go` stubs. To support a new OS add
  a build-tagged `Sample` — keep the render/parse core untouched.
- Hardcoded assumptions, kept intentionally simple (call them out if they
  bite): 4096-byte pages (`rss pages * 4`), and `cpu` reports process usage as
  a percentage of the whole CPU (normalized against the system-wide
  `/proc/stat` tick delta) rather than of one core.

## Debug logging

Logs go to a file, never stdout (stdout is shell code for `init` and the only
thing users pipe). Enabled only when `INCT_DEBUG` is truthy. File:
`$XDG_STATE_HOME/incantations/incantations.log` (`~/.local/state` on Linux,
`~/Library/Logs/incantations.log` on macOS), overridable with `INCT_LOG`.
Off by default → zero launch cost. `logutil.Init()` is idempotent and holds
the mutex, so log within `Init` via `logger.Printf` directly, not `Debugf`
(the classic self-deadlock).

## Testing conventions

- Golden files (`testdata/*.golden`) + `-update` flag for exact-output
  assertions (shell integration, render functions).
- Fixture files (`testdata/*.txt`) for parser inputs.
- Table-driven tests for dispatch and edge cases.
- `t.Setenv` for env-dependent `logutil` tests; never let logging pollute test
  stdout in ways that mask failure.

## Env knobs at a glance

- `INCT_DEBUG` — truthy enables debug logging to file.
- `INCT_LOG` — override the log file path (implies debug on).
- `INCT_*` otherwise unused; keep the surface small.