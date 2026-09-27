// Package format converts video files to another format with ffmpeg, showing
// an interactive picker of the supported targets or taking a target flag.
package format

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/metruzanca/incantations/internal/command"
	"golang.org/x/term"
)

// videoExts are the media kinds this command converts. Every one can become
// any other (the conversion table is symmetric: each source drops only
// itself), so a webm clip can become an mp4 or a gif, and a gif can become an
// mp4 again.
var videoExts = []string{"mp4", "webm", "mkv", "mov", "avi", "gif"}

// imageExts are the raster image formats this command converts between
// (symmetric, like the video table). svg is handled separately: ffmpeg can
// decode it but has no svg encoder, so it is only ever a source.
var imageExts = []string{"png", "jpg", "webp", "bmp", "tiff", "avif"}

// svgExt is a source-only image format. ffmpeg renders it to any raster image
// (with librsvg) but cannot encode svg, so nothing converts to it.
const svgExt = "svg"

// aliases maps non-canonical image extensions to the format they really are.
// A .jpeg and a .tif are the same data as a .jpg and a .tiff, so they convert
// as those.
var aliases = map[string]string{
	"jpeg": "jpg",
	"tif":  "tiff",
}

// normalize lowercases ext (with or without a leading dot) and resolves
// aliases, so jpg and jpeg are treated as one format.
func normalize(ext string) string {
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if a, ok := aliases[ext]; ok {
		return a
	}
	return ext
}

// isVideoExt reports whether ext (with or without a leading dot) is a
// supported video format.
func isVideoExt(ext string) bool {
	ext = normalize(ext)
	for _, e := range videoExts {
		if e == ext {
			return true
		}
	}
	return false
}

// isImageExt reports whether ext names a supported image format, including
// svg and the alias names (jpeg, tif).
func isImageExt(ext string) bool {
	ext = normalize(ext)
	if ext == svgExt {
		return true
	}
	for _, e := range imageExts {
		if e == ext {
			return true
		}
	}
	return false
}

// isTargetExt reports whether ext names a format this command can convert TO.
// Alias names resolve to their real format, so --jpeg works like --jpg; only
// svg has no encoder and is never a target.
func isTargetExt(ext string) bool {
	ext = normalize(ext)
	for _, e := range videoExts {
		if e == ext {
			return true
		}
	}
	for _, e := range imageExts {
		if e == ext {
			return true
		}
	}
	return false
}

// targetsFor returns the formats a source of the given extension can convert
// to: every format in its table except itself, in a fixed order. Images stay
// in the image table, so an svg becomes a png and a png never becomes a video.
func targetsFor(ext string) []string {
	ext = normalize(ext)
	table := videoExts
	if isImageExt(ext) {
		table = imageExts
	}
	out := make([]string, 0, len(table))
	for _, e := range table {
		if e != ext {
			out = append(out, e)
		}
	}
	return out
}

// opts carries the parsed command-line flags for one invocation.
type opts struct {
	file    string
	target  string // direct conversion target ext; "" means pick interactively
	discord bool   // pick mp4 (has audio) or gif (silent), what Discord likes
	list    bool   // print the choices without converting
}

// parseOpts parses flags and the single FILE argument. The target flags are
// the convert-to extensions (--mp4, --webm, --gif, --png, --jpg, ...);
// --discord and --list are also accepted.
func parseOpts(args []string) (opts, error) {
	var o opts
	for _, a := range args {
		switch {
		case strings.HasPrefix(a, "--"):
			name := strings.TrimPrefix(a, "--")
			switch {
			case name == "discord":
				o.discord = true
			case name == "list":
				o.list = true
			case isTargetExt(name):
				if o.target != "" {
					return o, fmt.Errorf("cannot convert to two formats (%s and %s)", o.target, name)
				}
				o.target = name
			default:
				return o, fmt.Errorf("unknown flag --%s (targets: --mp4 --webm --mkv --mov --avi --gif --png --jpg --webp --bmp --tiff --avif; also --discord and --list)", name)
			}
		case o.file != "":
			return o, fmt.Errorf("usage: incantations format [--mp4|--webm|--mkv|--mov|--avi|--gif|--png|--jpg|--webp|--bmp|--tiff|--avif|--discord] [--list] FILE")
		default:
			o.file = a
		}
	}
	switch {
	case o.file == "":
		return o, fmt.Errorf("usage: incantations format [--mp4|--webm|--mkv|--mov|--avi|--gif|--png|--jpg|--webp|--bmp|--tiff|--avif|--discord] [--list] FILE")
	case o.discord && o.target != "":
		return o, fmt.Errorf("cannot combine --discord with a target format (--%s)", o.target)
	case o.list && o.target != "":
		return o, fmt.Errorf("--list cannot be combined with a target format (--%s)", o.target)
	}
	return o, nil
}

// extOf returns the file's lowercase extension without the dot.
func extOf(file string) (string, error) {
	ext := strings.TrimPrefix(filepath.Ext(file), ".")
	if ext == "" {
		return "", fmt.Errorf("no file extension on %q; can't tell what kind of media it is", file)
	}
	return strings.ToLower(ext), nil
}

// outputPath is the conversion destination: same directory, same basename, new
// extension.
func outputPath(file, target string) string {
	ext := filepath.Ext(file)
	base := strings.TrimSuffix(filepath.Base(file), ext)
	return filepath.Join(filepath.Dir(file), base+"."+target)
}

// ffmpegArgs builds the ffmpeg invocation, letting ffmpeg pick the right
// encoder for the target container. -stats streams progress to stderr; -y
// overwrites an existing output silently.
func ffmpegArgs(in, out string) []string {
	return []string{"-hide_banner", "-loglevel", "warning", "-stats", "-y", "-i", in, out}
}

// runFFmpeg executes the conversion. A package var so tests stub it out (the
// same pattern as ports.signalProcess).
var runFFmpeg = func(in, out string) error {
	ff, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg not found in PATH; install ffmpeg to use format")
	}
	cmd := exec.Command(ff, ffmpegArgs(in, out)...)
	cmd.Stdout = os.Stderr // keep stdout clean for the "wrote" line
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runProbe executes ffprobe for --discord audio detection and returns its
// stdout. A package var so tests stub it out.
var runProbe = func(file string) (string, error) {
	ff, err := exec.LookPath("ffprobe")
	if err != nil {
		return "", fmt.Errorf("ffprobe not found in PATH; --discord needs it (it ships with ffmpeg)")
	}
	cmd := exec.Command(ff, "-v", "error", "-select_streams", "a", "-show_entries", "stream=index", "-of", "csv=p=0", file)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("could not read %q as a media file: %w", file, err)
	}
	return string(out), nil
}

// hasAudioStream reports whether ffprobe listed any audio streams.
func hasAudioStream(probe string) bool {
	return strings.TrimSpace(probe) != ""
}

// discordDecision picks the target Discord likes best: mp4 when the video has
// audio, gif when it is silent. The reason names the probe result for the
// --list view.
func discordDecision(file string) (target, reason string, err error) {
	probe, err := runProbe(file)
	if err != nil {
		return "", "", err
	}
	if hasAudioStream(probe) {
		return "mp4", "has audio", nil
	}
	return "gif", "silent (no audio)", nil
}

// Spec registers the format command.
func Spec() command.Entry {
	return command.Entry{
		Name:    "format",
		Summary: "convert a video or image to another format (mp4, webm, mkv, mov, avi, gif, png, jpg, webp, bmp, tiff, avif)",
		Help: `Usage:
  incantations format FILE
  incantations format --gif FILE        # or --mp4 --webm --mkv --mov --avi --png --jpg --webp --bmp --tiff --avif
  incantations format --discord FILE    # mp4 if it has audio, gif if silent
  incantations format --list FILE

Shows a dropdown of the formats FILE can convert to and converts the one you
pick, using ffmpeg (which must be installed). Videos convert to videos and
images to images: an svg becomes a png, a png becomes a jpg, and so on (svg
needs an ffmpeg built with the svg decoder, e.g. librsvg). The result lands
next to FILE with the new extension, overwriting an existing file silently.

FILE may be a glob, so incantations format "downloads/*.webp" --png converts
every match. Quote the pattern so the shell passes it through; the dropdown
only works with a single file, so a target flag (or --discord/--list) is
required when the pattern matches more than one.

Pass a target flag to skip the dropdown, --discord to pick what Discord wants
(mp4 for a video with audio, gif for a silent one), or --list to print the
choices without converting. The dropdown needs a terminal; when piped, use a
target flag instead.`,
		Run: func(args []string, stdout io.Writer) error {
			return run(args, stdout)
		},
	}
}

// expandFiles resolves the FILE argument, which may be a shell glob (typically
// quoted, e.g. "downloads/*.webp"). A pattern with no metacharacters is passed
// through untouched so a missing literal file keeps its precise stat error; a
// real pattern must match at least one file. Matches come from filepath.Glob,
// which already sorts them.
func expandFiles(arg string) ([]string, error) {
	if !strings.ContainsAny(arg, "*?[") {
		return []string{arg}, nil
	}
	matches, err := filepath.Glob(arg)
	if err != nil {
		return nil, fmt.Errorf("bad pattern %q: %w", arg, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no files match %q", arg)
	}
	return matches, nil
}

// run executes one format invocation over every file the FILE argument names.
func run(args []string, stdout io.Writer) error {
	o, err := parseOpts(args)
	if err != nil {
		return err
	}
	files, err := expandFiles(o.file)
	if err != nil {
		return err
	}
	if len(files) > 1 && !o.list && o.target == "" && !o.discord {
		return fmt.Errorf("%d files match %q; the dropdown converts one at a time, pass a target flag (--png, --gif, ...) to convert them all", len(files), o.file)
	}
	for _, file := range files {
		if err := runOne(stdout, o, file); err != nil {
			return err
		}
	}
	return nil
}

// runOne converts or lists a single resolved file.
func runOne(stdout io.Writer, o opts, file string) error {
	if _, err := os.Stat(file); err != nil {
		return fmt.Errorf("%s: %w", file, err)
	}
	ext, err := extOf(file)
	if err != nil {
		return err
	}
	if !isVideoExt(ext) && !isImageExt(ext) {
		return fmt.Errorf("%q is not a supported media format (video: mp4, webm, mkv, mov, avi, gif; image: png, jpg, webp, svg, bmp, tiff, avif)", file)
	}
	if o.discord && isImageExt(ext) {
		return fmt.Errorf("%s is an image; --discord only applies to videos", file)
	}

	switch {
	case o.list:
		return listTargets(stdout, file, ext, o)
	case o.target != "":
		if normalize(o.target) == normalize(ext) {
			return fmt.Errorf("%s is already .%s; nothing to convert", file, ext)
		}
		return convertFile(stdout, file, o.target)
	case o.discord:
		target, reason, err := discordDecision(file)
		if err != nil {
			return err
		}
		if normalize(target) == normalize(ext) {
			return fmt.Errorf("%s is already .%s; nothing to convert for discord", file, ext)
		}
		fmt.Fprintf(stdout, "%s \u2192 .%s (%s)\n", filepath.Base(file), target, reason)
		return convertFile(stdout, file, target)
	default:
		return interactive(stdout, file, ext)
	}
}

// convertFile runs ffmpeg and reports the output path.
func convertFile(w io.Writer, file, target string) error {
	out := outputPath(file, target)
	if err := runFFmpeg(file, out); err != nil {
		return fmt.Errorf("conversion to .%s failed: %w", target, err)
	}
	_, err := fmt.Fprintf(w, "wrote %s\n", out)
	return err
}

// listTargets prints the conversion choices without converting.
func listTargets(w io.Writer, file, ext string, o opts) error {
	if o.discord {
		target, reason, err := discordDecision(file)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%s \u2192 .%s (%s)\n", file, target, reason)
		return err
	}
	ts := targetsFor(ext)
	dotted := make([]string, len(ts))
	for i, t := range ts {
		dotted[i] = "." + t
	}
	_, err := fmt.Fprintf(w, "%s \u2192 %s\n", file, strings.Join(dotted, ", "))
	return err
}

// interactive opens the dropdown picker. The picker needs a real terminal, so
// when stdin is piped we point the user at the direct flags instead.
func interactive(stdout io.Writer, file, ext string) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return fmt.Errorf("the format dropdown needs a terminal; pass a target flag (--gif, --mp4, ...) or --discord to convert directly, or --list to see the choices")
	}
	target, err := pick(targetsFor(ext), file)
	if err != nil {
		return err
	}
	return convertFile(stdout, file, target)
}