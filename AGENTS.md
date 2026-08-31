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

## Layout

- `cmd/incantations/main.go` — thin entrypoint. Logging init + `app.Run`.
- `internal/app` — dispatch. Owns the `command.Registry`, help/version/errors,
  exit codes (0 ok, 1 runtime error, 2 usage error). `New` is variadic so
  tests inject synthetic entries. `Version` is overridable via
  `-ldflags "-X github.com/metruzanca/incantations/internal/app.Version=vX"`.
- `internal/command` — tiny registry (`Entry{Name, Summary, Meta, Run}`).
  The single source of truth for dispatch and for `init` shell generation.
- `internal/shell` — `Generate(shell, commands)` is pure and deterministic.
  bash/zsh share one template; fish differs (`$argv`). Golden files live in
  `testdata/`; regenerate with `-update`. `init` accepts normalized names
  (`/bin/zsh`, `zsh`, `ZSH` → `zsh`).
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
4. `Spec()` returns a `command.Entry` wired to `Sample` + `Render`.

To add a command: create `internal/<name>`, register its `Spec()` in
`app.New`, add fixture/golden tests. No shell-config work: `init` regenerates
functions from the registry automatically.

## Platform contract

- Linux reads `/proc` directly (no cgo, no external deps): `meminfo`,
  `/proc/<pid>/status`, `/proc/stat`, `/proc/<pid>/stat`, `/proc/loadavg`.
- Process display names come from the first token of `/proc/<pid>/cmdline`
  argv[0] (falling back to `comm`, which the kernel truncates to 15 chars).
  `ram` groups same-named processes (summed RSS + count); `cpu` lists per PID.
  The `/proc/self/exe` argv[0] linker trick is treated as no name.
- `disk` shells out to `df -hT` on any non-Windows platform; `parseDf` is the
  pure, testable core. Virtual filesystems are filtered via `hiddenTypes`
  (tmpfs/overlay/proc/etc.).
- Non-Linux `ram`/`cpu` use `*_unsupported.go` stubs. To support a new OS add
  a build-tagged `Sample` — keep the render/parse core untouched.
- Hardcoded assumptions, kept intentionally simple (call them out if they
  bite): `userHZ = 100` ticks/sec, 4096-byte pages (`rss pages * 4`).

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