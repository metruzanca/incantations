package format

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseOpts(t *testing.T) {
	cases := []struct {
		args    []string
		want    opts
		wantErr bool
	}{
		{[]string{"clip.webm"}, opts{file: "clip.webm"}, false},
		{[]string{"--gif", "clip.webm"}, opts{file: "clip.webm", target: "gif"}, false},
		{[]string{"--mp4", "clip.webm"}, opts{file: "clip.webm", target: "mp4"}, false},
		{[]string{"--discord", "clip.webm"}, opts{file: "clip.webm", discord: true}, false},
		{[]string{"--list", "clip.webm"}, opts{file: "clip.webm", list: true}, false},
		{[]string{"--png", "icon.svg"}, opts{file: "icon.svg", target: "png"}, false},
		{[]string{"--svg", "icon.png"}, opts{}, true}, // svg is source-only, never a target
		{[]string{"--discord", "--list", "clip.webm"}, opts{file: "clip.webm", discord: true, list: true}, false},
		{[]string{"--discord", "--gif", "clip.webm"}, opts{}, true},  // conflicting modes
		{[]string{"--list", "--gif", "clip.webm"}, opts{}, true},     // list + target
		{[]string{"--mp4", "--gif", "clip.webm"}, opts{}, true},      // two targets
		{[]string{"--bogus", "clip.webm"}, opts{}, true},             // unknown flag
		{[]string{}, opts{}, true},                                   // no file
		{[]string{"--gif"}, opts{}, true},                            // flag, no file
		{[]string{"a.mp4", "b.mp4"}, opts{}, true},                   // two files
	}
	for _, tc := range cases {
		got, err := parseOpts(tc.args)
		if (err != nil) != tc.wantErr {
			t.Errorf("parseOpts(%v) err = %v, wantErr %v", tc.args, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("parseOpts(%v) = %+v, want %+v", tc.args, got, tc.want)
		}
	}
}

func TestTargetsFor(t *testing.T) {
	cases := map[string][]string{
		"webm": {"mp4", "mkv", "mov", "avi", "gif"},
		"mp4":  {"webm", "mkv", "mov", "avi", "gif"},
		"gif":  {"mp4", "webm", "mkv", "mov", "avi"},
		"MOV":  {"mp4", "webm", "mkv", "avi", "gif"},
		".avi": {"mp4", "webm", "mkv", "mov", "gif"},
		"png":  {"jpg", "webp", "bmp", "tiff", "avif"},
		"jpg":  {"png", "webp", "bmp", "tiff", "avif"},
		"jpeg": {"png", "webp", "bmp", "tiff", "avif"},
		"svg":  {"png", "jpg", "webp", "bmp", "tiff", "avif"},
	}
	for in, want := range cases {
		if got := targetsFor(in); strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("targetsFor(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestIsVideoExt(t *testing.T) {
	for _, ext := range []string{"webm", "MP4", ".mkv", "gif"} {
		if !isVideoExt(ext) {
			t.Errorf("isVideoExt(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{"png", "svg", "mp3", "", ".txt"} {
		if isVideoExt(ext) {
			t.Errorf("isVideoExt(%q) = true, want false", ext)
		}
	}
}

func TestIsImageExt(t *testing.T) {
	for _, ext := range []string{"png", "JPG", ".svg", "jpeg", "webp", "bmp", "tiff", "tif"} {
		if !isImageExt(ext) {
			t.Errorf("isImageExt(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{"mp4", "gif", "txt", ""} {
		if isImageExt(ext) {
			t.Errorf("isImageExt(%q) = true, want false", ext)
		}
	}
}

func TestIsTargetExt(t *testing.T) {
	for _, ext := range []string{"mp4", "gif", "png", "jpg", "jpeg", "webp", "bmp", "tiff", "tif", "avif"} {
		if !isTargetExt(ext) {
			t.Errorf("isTargetExt(%q) = false, want true", ext)
		}
	}
	for _, ext := range []string{"svg", "txt"} {
		if isTargetExt(ext) {
			t.Errorf("isTargetExt(%q) = true, want false", ext)
		}
	}
}

func TestExtOf(t *testing.T) {
	cases := map[string]string{
		"clip.webm":   "webm",
		"a/Clip.MP4":  "mp4",
		".hidden.mov": "mov",
	}
	for in, want := range cases {
		if got, err := extOf(in); err != nil || got != want {
			t.Errorf("extOf(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := extOf("noext"); err == nil {
		t.Error("extOf(noext) should error")
	}
}

func TestOutputPath(t *testing.T) {
	if got := outputPath("clip.webm", "gif"); got != "clip.gif" {
		t.Errorf("outputPath = %q, want clip.gif", got)
	}
	if got := outputPath("/videos/clip.webm", "mp4"); got != "/videos/clip.mp4" {
		t.Errorf("outputPath = %q, want /videos/clip.mp4", got)
	}
	if got := outputPath("clip.WEBM", "gif"); got != "clip.gif" {
		t.Errorf("outputPath = %q, want clip.gif", got)
	}
}

func TestFFmpegArgs(t *testing.T) {
	args := strings.Join(ffmpegArgs("in.webm", "out.mp4"), " ")
	for _, want := range []string{"-i", "in.webm", "out.mp4", "-y", "-hide_banner"} {
		if !strings.Contains(args, want) {
			t.Errorf("ffmpegArgs missing %q: %s", want, args)
		}
	}
}

func TestHasAudioStream(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"\n":    false,
		"0\n":   true,
		"0\n1\n": true,
	}
	for in, want := range cases {
		if got := hasAudioStream(in); got != want {
			t.Errorf("hasAudioStream(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestDiscordDecision(t *testing.T) {
	old := runProbe
	defer func() { runProbe = old }()
	runProbe = func(string) (string, error) { return "0\n", nil }
	if target, reason, err := discordDecision("x"); err != nil || target != "mp4" || !strings.Contains(reason, "audio") {
		t.Errorf("discordDecision(audio) = %q %q %v", target, reason, err)
	}
	runProbe = func(string) (string, error) { return "", nil }
	if target, reason, err := discordDecision("x"); err != nil || target != "gif" || !strings.Contains(reason, "silent") {
		t.Errorf("discordDecision(silent) = %q %q %v", target, reason, err)
	}
	runProbe = func(string) (string, error) { return "", io.ErrUnexpectedEOF }
	if _, _, err := discordDecision("x"); err == nil {
		t.Error("discordDecision should surface probe errors")
	}
}

// tmpVideo writes an empty placeholder media file in a temp dir.
func tmpVideo(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRunDirectConvert(t *testing.T) {
	file := tmpVideo(t, "clip.webm")
	old := runFFmpeg
	var gotIn, gotOut string
	runFFmpeg = func(in, out string) error { gotIn, gotOut = in, out; return nil }
	defer func() { runFFmpeg = old }()
	var out strings.Builder
	if err := run([]string{"--gif", file}, &out); err != nil {
		t.Fatal(err)
	}
	if gotIn != file || gotOut != filepath.Join(filepath.Dir(file), "clip.gif") {
		t.Errorf("ffmpeg called with (%s, %s), want (%s, clip.gif)", gotIn, gotOut, file)
	}
	if !strings.Contains(out.String(), "wrote") {
		t.Errorf("output missing wrote line:\n%s", out.String())
	}
}

func TestRunConvertErrors(t *testing.T) {
	if err := run([]string{"--gif", "missing.webm"}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "missing.webm") {
		t.Errorf("missing file should error, got %v", err)
	}
	file := tmpVideo(t, "clip.mp4")
	if err := run([]string{"--mp4", file}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already .mp4") {
		t.Errorf("same-format target should error, got %v", err)
	}
	file = tmpVideo(t, "clip.txt")
	if err := run([]string{"--gif", file}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "not a supported media format") {
		t.Errorf("unsupported source should error, got %v", err)
	}
}

func TestRunImageConvert(t *testing.T) {
	file := tmpVideo(t, "icon.svg")
	old := runFFmpeg
	var gotIn, gotOut string
	runFFmpeg = func(in, out string) error { gotIn, gotOut = in, out; return nil }
	defer func() { runFFmpeg = old }()
	var out strings.Builder
	if err := run([]string{"--png", file}, &out); err != nil {
		t.Fatal(err)
	}
	if gotIn != file || gotOut != filepath.Join(filepath.Dir(file), "icon.png") {
		t.Errorf("ffmpeg called with (%s, %s), want (%s, icon.png)", gotIn, gotOut, file)
	}
	if !strings.Contains(out.String(), "wrote") {
		t.Errorf("output missing wrote line:\n%s", out.String())
	}
}

func TestRunImageErrors(t *testing.T) {
	file := tmpVideo(t, "icon.png")
	if err := run([]string{"--png", file}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already .png") {
		t.Errorf("same-format image should error, got %v", err)
	}
	file = tmpVideo(t, "icon.jpeg")
	if err := run([]string{"--jpg", file}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already .jpeg") {
		t.Errorf("alias same-format should error, got %v", err)
	}
	file = tmpVideo(t, "icon.png")
	if err := run([]string{"--discord", file}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "image") {
		t.Errorf("--discord on an image should error, got %v", err)
	}
}

func TestRunDiscord(t *testing.T) {
	oldP, oldF := runProbe, runFFmpeg
	defer func() { runProbe, runFFmpeg = oldP, oldF }()

	file := tmpVideo(t, "clip.webm")
	var gotOut string
	runFFmpeg = func(in, out string) error { gotOut = out; return nil }

	runProbe = func(string) (string, error) { return "0\n", nil }
	var out strings.Builder
	if err := run([]string{"--discord", file}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotOut, "clip.mp4") {
		t.Errorf("discord(audio) should convert to mp4, got %s", gotOut)
	}
	if !strings.Contains(out.String(), "has audio") {
		t.Errorf("discord output should state the reason:\n%s", out.String())
	}

	runProbe = func(string) (string, error) { return "", nil }
	out.Reset()
	if err := run([]string{"--discord", file}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(gotOut, "clip.gif") {
		t.Errorf("discord(silent) should convert to gif, got %s", gotOut)
	}

	runProbe = func(string) (string, error) { return "", io.ErrUnexpectedEOF }
	if err := run([]string{"--discord", file}, io.Discard); err == nil {
		t.Error("discord should surface probe errors")
	}

	// A source that is already the decided target is a no-op, not a clobber.
	file = tmpVideo(t, "clip.mp4")
	runProbe = func(string) (string, error) { return "0\n", nil }
	if err := run([]string{"--discord", file}, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "already .mp4") {
		t.Errorf("discord same-format should error, got %v", err)
	}
}

func TestRunList(t *testing.T) {
	file := tmpVideo(t, "clip.webm")
	var out strings.Builder
	if err := run([]string{"--list", file}, &out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"clip.webm", ".mp4", ".gif", ".mkv"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("list missing %q:\n%s", want, out.String())
		}
	}
	if i := strings.Index(out.String(), "\u2192"); i >= 0 {
		if strings.Contains(out.String()[i:], ".webm") {
			t.Errorf("list should not offer the source format:\n%s", out.String())
		}
	} else {
		t.Errorf("list output missing arrow:\n%s", out.String())
	}

	old := runProbe
	defer func() { runProbe = old }()
	runProbe = func(string) (string, error) { return "", nil }
	out.Reset()
	if err := run([]string{"--discord", "--list", file}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), ".gif") || !strings.Contains(out.String(), "silent") {
		t.Errorf("discord --list should show the decision:\n%s", out.String())
	}
}