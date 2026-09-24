package tag

import (
	"io"
	"os"
	"strings"
	"testing"
)

// fakeGit replaces gitRun for the duration of a test and records the commands.
func fakeGit(t *testing.T, handler func(args ...string) (string, error)) *[][]string {
	t.Helper()
	calls := &[][]string{}
	old := gitRun
	gitRun = func(args ...string) (string, error) {
		*calls = append(*calls, args)
		return handler(args...)
	}
	t.Cleanup(func() { gitRun = old })
	return calls
}

func TestParseOpts(t *testing.T) {
	cases := []struct {
		args    []string
		want    opts
		wantErr bool
	}{
		{[]string{}, opts{}, false},
		{[]string{"v1.2.3"}, opts{name: "v1.2.3"}, false},
		{[]string{"--push", "v1.2.3"}, opts{name: "v1.2.3", push: true}, false},
		{[]string{"v1.2.3", "--push"}, opts{name: "v1.2.3", push: true}, false},
		{[]string{"--push"}, opts{}, true},          // push without a name
		{[]string{"a", "b"}, opts{}, true},          // two names
		{[]string{"--bogus"}, opts{}, true},         // unknown flag
		{[]string{"v1.2.3", "extra"}, opts{}, true}, // name then stray
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

func TestCreateTagLocal(t *testing.T) {
	calls := fakeGit(t, func(args ...string) (string, error) { return "", nil })
	var out strings.Builder
	if err := createTag("v1.2.3", false, &out); err != nil {
		t.Fatal(err)
	}
	if len(*calls) != 1 || strings.Join((*calls)[0], " ") != "tag v1.2.3" {
		t.Errorf("git calls = %v, want [tag v1.2.3]", *calls)
	}
	if !strings.Contains(out.String(), "Created tag v1.2.3") || !strings.Contains(out.String(), "git push origin v1.2.3") {
		t.Errorf("output should confirm creation and show the push hint:\n%s", out.String())
	}
}

func TestCreateTagEmpty(t *testing.T) {
	fakeGit(t, func(args ...string) (string, error) { return "", nil })
	if err := createTag("   ", false, io.Discard); err == nil {
		t.Error("empty tag name should error")
	}
}

func TestCreateTagPushPushesCommitsFirst(t *testing.T) {
	calls := fakeGit(t, func(args ...string) (string, error) {
		if strings.Join(args, " ") == "rev-parse --abbrev-ref HEAD" {
			return "main\n", nil
		}
		return "", nil
	})
	var out strings.Builder
	if err := createTag("v1.2.3", true, &out); err != nil {
		t.Fatal(err)
	}
	got := make([]string, len(*calls))
	for i, c := range *calls {
		got[i] = strings.Join(c, " ")
	}
	want := []string{"tag v1.2.3", "rev-parse --abbrev-ref HEAD", "push origin main", "push origin v1.2.3"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("git calls = %v, want %v", got, want)
	}
	if !strings.Contains(out.String(), "Pushed tag v1.2.3") {
		t.Errorf("output = %q", out.String())
	}
}

func TestPushDetachedHeadRejectsUnpushedCommit(t *testing.T) {
	fakeGit(t, func(args ...string) (string, error) {
		switch strings.Join(args, " ") {
		case "rev-parse --abbrev-ref HEAD":
			return "HEAD\n", nil
		case "branch -r --contains v1.2.3":
			return "", nil // not on any remote branch
		}
		return "", nil
	})
	if err := createTag("v1.2.3", true, io.Discard); err == nil ||
		!strings.Contains(err.Error(), "detached HEAD") {
		t.Errorf("detached HEAD with unpushed commit should error, got %v", err)
	}
}

func TestListTags(t *testing.T) {
	fakeGit(t, func(args ...string) (string, error) {
		return "v0.1.0\nv1.10.0\nv1.9.0\nrelease\n", nil
	})
	var out strings.Builder
	if err := listTags(&out); err != nil {
		t.Fatal(err)
	}
	want := "v1.10.0\nv1.9.0\nv0.1.0\nrelease\n"
	if out.String() != want {
		t.Errorf("listTags = %q, want %q", out.String(), want)
	}
}

func TestListTagsEmpty(t *testing.T) {
	fakeGit(t, func(args ...string) (string, error) { return "", nil })
	var out strings.Builder
	if err := listTags(&out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "No tags.\n" {
		t.Errorf("empty list = %q, want %q", out.String(), "No tags.\n")
	}
}

func TestRunPipedListsTags(t *testing.T) {
	fakeGit(t, func(args ...string) (string, error) { return "v0.1.0\nv0.2.0\n", nil })
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	defer w.Close()
	var out strings.Builder
	if err := run(nil, r, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "v0.2.0\nv0.1.0\n" {
		t.Errorf("piped tag = %q", out.String())
	}
}
