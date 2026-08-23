package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestParsePorcelain(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []DirtyFile
	}{
		{
			name: "unstaged modified",
			in:   " M internal/git/git.go",
			want: []DirtyFile{{Status: "M", Path: "internal/git/git.go"}},
		},
		{
			name: "staged modified",
			in:   "M  staged.go",
			want: []DirtyFile{{Status: "M", Path: "staged.go"}},
		},
		{
			name: "untracked",
			in:   "?? newfile.txt",
			want: []DirtyFile{{Status: "??", Path: "newfile.txt"}},
		},
		{
			name: "staged added",
			in:   "A  added.go",
			want: []DirtyFile{{Status: "A", Path: "added.go"}},
		},
		{
			name: "rename with arrow",
			in:   "R  old.go -> new.go",
			want: []DirtyFile{{Status: "R", Path: "new.go"}},
		},
		{
			name: "deleted",
			in:   "D  gone.go",
			want: []DirtyFile{{Status: "D", Path: "gone.go"}},
		},
		{
			name: "multiple files",
			in:   " M foo.go\n?? bar.txt\nA  baz.go",
			want: []DirtyFile{
				{Status: "M", Path: "foo.go"},
				{Status: "??", Path: "bar.txt"},
				{Status: "A", Path: "baz.go"},
			},
		},
		{
			name: "short line",
			in:   "M",
			want: []DirtyFile{{Path: "M"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePorcelain(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d entries, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].Status != tt.want[i].Status {
					t.Errorf("[%d] status = %q, want %q", i, got[i].Status, tt.want[i].Status)
				}
				if got[i].Path != tt.want[i].Path {
					t.Errorf("[%d] path = %q, want %q", i, got[i].Path, tt.want[i].Path)
				}
			}
		})
	}
}

// fakeGhBinary creates a minimal fake gh CLI in dir that responds to the
// commands used by OpenIssues, OpenPRs, and RepoDescription. On Windows it
// writes a .cmd batch file; on Unix it writes an executable shell script.
func fakeGhBinary(dir string) string {
	bin := "gh"
	if runtime.GOOS == "windows" {
		bin = "gh.cmd"
	}
	path := filepath.Join(dir, bin)

	var script string
	if runtime.GOOS == "windows" {
		script = `@echo off
if "%1"=="issue" echo [{"number":1,"title":"Fake issue","url":"https://github.com/fake/repo/issues/1"}]
if "%1"=="pr" echo [{"number":2,"title":"Fake PR","url":"https://github.com/fake/repo/pull/2"}]
if "%1"=="repo" echo {"description":"Fake repo description"}
`
	} else {
		script = `#!/bin/sh
case "$1" in
  issue) echo '[{"number":1,"title":"Fake issue","url":"https://github.com/fake/repo/issues/1"}]' ;;
  pr)    echo '[{"number":2,"title":"Fake PR","url":"https://github.com/fake/repo/pull/2"}]' ;;
  repo)  echo '{"description":"Fake repo description"}' ;;
esac
`
	}

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		panic(err)
	}
	return path
}

// setPathFirst prepends dir to the PATH environment variable for the duration
// of the test. It restores the original PATH on cleanup.
func setPathFirst(t *testing.T, dir string) {
	t.Helper()
	orig := os.Getenv("PATH")
	newPath := dir + string(os.PathListSeparator) + orig
	os.Setenv("PATH", newPath)
	t.Cleanup(func() { os.Setenv("PATH", orig) })
}

func TestOpenIssues(t *testing.T) {
	dir := t.TempDir()
	fakeGhBinary(dir)
	setPathFirst(t, dir)

	got := OpenIssues(dir)
	if len(got) != 1 {
		t.Fatalf("got %d issues, want 1", len(got))
	}
	if got[0].Number != 1 {
		t.Errorf("number = %d, want 1", got[0].Number)
	}
	if got[0].Title != "Fake issue" {
		t.Errorf("title = %q, want %q", got[0].Title, "Fake issue")
	}
	if got[0].URL != "https://github.com/fake/repo/issues/1" {
		t.Errorf("url = %q, want %q", got[0].URL, "https://github.com/fake/repo/issues/1")
	}
}

func TestOpenPRs(t *testing.T) {
	dir := t.TempDir()
	fakeGhBinary(dir)
	setPathFirst(t, dir)

	got := OpenPRs(dir)
	if len(got) != 1 {
		t.Fatalf("got %d PRs, want 1", len(got))
	}
	if got[0].Number != 2 {
		t.Errorf("number = %d, want 2", got[0].Number)
	}
	if got[0].Title != "Fake PR" {
		t.Errorf("title = %q, want %q", got[0].Title, "Fake PR")
	}
	if got[0].URL != "https://github.com/fake/repo/pull/2" {
		t.Errorf("url = %q, want %q", got[0].URL, "https://github.com/fake/repo/pull/2")
	}
}

func TestRepoDescription(t *testing.T) {
	dir := t.TempDir()
	fakeGhBinary(dir)
	setPathFirst(t, dir)

	got := RepoDescription(dir)
	if got != "Fake repo description" {
		t.Errorf("description = %q, want %q", got, "Fake repo description")
	}
}

func TestOpenIssues_NoGh(t *testing.T) {
	orig := os.Getenv("PATH")
	empty := t.TempDir()
	os.Setenv("PATH", empty)
	t.Cleanup(func() { os.Setenv("PATH", orig) })

	got := OpenIssues(t.TempDir())
	if got != nil {
		t.Errorf("expected nil when gh not available, got %v", got)
	}
}

func TestOpenPRs_NoGh(t *testing.T) {
	orig := os.Getenv("PATH")
	empty := t.TempDir()
	os.Setenv("PATH", empty)
	t.Cleanup(func() { os.Setenv("PATH", orig) })

	got := OpenPRs(t.TempDir())
	if got != nil {
		t.Errorf("expected nil when gh not available, got %v", got)
	}
}

func TestRepoDescription_NoGh(t *testing.T) {
	orig := os.Getenv("PATH")
	empty := t.TempDir()
	os.Setenv("PATH", empty)
	t.Cleanup(func() { os.Setenv("PATH", orig) })

	got := RepoDescription(t.TempDir())
	if got != "" {
		t.Errorf("expected empty string when gh not available, got %q", got)
	}
}

func TestOpenIssues_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bin := "gh"
	if runtime.GOOS == "windows" {
		bin = "gh.cmd"
	}
	path := filepath.Join(dir, bin)
	if runtime.GOOS == "windows" {
		_ = os.WriteFile(path, []byte("@echo off\necho not-json\n"), 0o755)
	} else {
		_ = os.WriteFile(path, []byte("#!/bin/sh\necho not-json\n"), 0o755)
	}
	setPathFirst(t, dir)

	got := OpenIssues(dir)
	if got != nil {
		t.Errorf("expected nil for invalid JSON, got %v", got)
	}
}

func TestOpenPRs_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bin := "gh"
	if runtime.GOOS == "windows" {
		bin = "gh.cmd"
	}
	path := filepath.Join(dir, bin)
	if runtime.GOOS == "windows" {
		_ = os.WriteFile(path, []byte("@echo off\necho not-json\n"), 0o755)
	} else {
		_ = os.WriteFile(path, []byte("#!/bin/sh\necho not-json\n"), 0o755)
	}
	setPathFirst(t, dir)

	got := OpenPRs(dir)
	if got != nil {
		t.Errorf("expected nil for invalid JSON, got %v", got)
	}
}

func TestRepoDescription_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	bin := "gh"
	if runtime.GOOS == "windows" {
		bin = "gh.cmd"
	}
	path := filepath.Join(dir, bin)
	if runtime.GOOS == "windows" {
		_ = os.WriteFile(path, []byte("@echo off\necho not-json\n"), 0o755)
	} else {
		_ = os.WriteFile(path, []byte("#!/bin/sh\necho not-json\n"), 0o755)
	}
	setPathFirst(t, dir)

	got := RepoDescription(dir)
	if got != "" {
		t.Errorf("expected empty string for invalid JSON, got %q", got)
	}
}

// ── Ported from upstream: additional tests ──

func initTestRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	mustGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	mustGit("init")
	mustGit("config", "user.email", "test@example.com")
	mustGit("config", "user.name", "Test")
	mustGit("commit", "--allow-empty", "-m", "init")
	mustGit("branch", "-M", "main")
	return dir
}

func TestDescribeDefaultBranch(t *testing.T) {
	dir := initTestRepo(t)

	info, err := Describe(dir)
	if err != nil {
		t.Fatalf("Describe: %v", err)
	}
	if !info.IsRepo || info.Branch != "main" {
		t.Fatalf("got %+v, want repo on main", info)
	}
	if !info.IsDefault {
		t.Fatalf("main should be reported as the default branch")
	}

	// The default branch must not be renamable, even via the API directly.
	if _, err := RenameBranch(dir, "renamed-main"); err != ErrDefaultBranch {
		t.Fatalf("renaming default branch: got %v, want ErrDefaultBranch", err)
	}
	if info, _ := Describe(dir); info.Branch != "main" {
		t.Fatalf("default branch was renamed to %q despite guard", info.Branch)
	}

	// Create and switch to a feature branch so the rename below is allowed.
	cmd := exec.Command("git", "checkout", "-b", "feature/tmp")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("checkout feature branch: %v (%s)", err, out)
	}

	if _, err := RenameBranch(dir, "feature/x"); err != nil {
		t.Fatalf("RenameBranch: %v", err)
	}
	info, _ = Describe(dir)
	if info.Branch != "feature/x" {
		t.Fatalf("got branch %q, want feature/x", info.Branch)
	}
	if info.IsDefault {
		t.Fatalf("feature/x should not be the default branch")
	}
}

func TestDescribeNonRepo(t *testing.T) {
	info, err := Describe(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("Describe non-repo returned error: %v", err)
	}
	if info.IsRepo {
		t.Fatalf("expected IsRepo false for non-repo dir")
	}
}

func TestWorkingTreeDiff(t *testing.T) {
	dir := initTestRepo(t)
	mustGit := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	// Clean tree: empty diff.
	if out, err := WorkingTreeDiff(dir); err != nil || out != "" {
		t.Fatalf("clean tree: got %q, %v; want empty", out, err)
	}

	// Tracked modification.
	write("tracked.txt", "line1\nline2\n")
	mustGit("add", "tracked.txt")
	mustGit("commit", "-m", "add tracked")
	write("tracked.txt", "line1\nCHANGED\n")
	// Untracked new file.
	write("untracked.txt", "brand new\n")

	out, err := WorkingTreeDiff(dir)
	if err != nil {
		t.Fatalf("WorkingTreeDiff: %v", err)
	}
	for _, want := range []string{
		"b/tracked.txt", "-line2", "+CHANGED",
		"b/untracked.txt", "new file mode", "+brand new",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("diff missing %q:\n%s", want, out)
		}
	}
}

func TestWorkingTreeDiffNonRepo(t *testing.T) {
	if _, err := WorkingTreeDiff(filepath.Join(t.TempDir(), "nope")); err != ErrNotRepo {
		t.Fatalf("got %v, want ErrNotRepo", err)
	}
}

func TestWorkingTreeDiffBinaryUntracked(t *testing.T) {
	dir := initTestRepo(t)
	if err := os.WriteFile(filepath.Join(dir, "blob.bin"), []byte{0x00, 0x01, 0x02, 0x00}, 0644); err != nil {
		t.Fatal(err)
	}
	out, err := WorkingTreeDiff(dir)
	if err != nil {
		t.Fatalf("WorkingTreeDiff: %v", err)
	}
	if !strings.Contains(out, "b/blob.bin") || !strings.Contains(out, "Binary files") {
		t.Fatalf("expected a binary marker for blob.bin:\n%s", out)
	}
}

func TestWorkingTreeDiffBoundedOnManyUntracked(t *testing.T) {
	dir := initTestRepo(t)
	big := strings.Repeat("a line of content that is reasonably long\n", 400) // ~17 KiB each
	for i := 0; i < 1000; i++ {
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("u%04d.txt", i)), []byte(big), 0644); err != nil {
			t.Fatal(err)
		}
	}
	start := time.Now()
	out, err := WorkingTreeDiff(dir)
	if err != nil {
		t.Fatalf("WorkingTreeDiff: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("WorkingTreeDiff took %v on 1000 untracked files; expected it to be fast", elapsed)
	}
	if len(out) > maxDiffBytes+(64<<10) {
		t.Fatalf("output %d bytes exceeds the cap %d", len(out), maxDiffBytes)
	}
}

func TestValidBranchName(t *testing.T) {
	valid := []string{
		"main",
		"feature/pr-button",
		"fix_123",
		"release-2.1.0",
		"a",
	}
	for _, name := range valid {
		if !ValidBranchName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalid := []string{
		"",
		"-leading-dash",
		"/leading-slash",
		"trailing-slash/",
		"has space",
		"double..dot",
		"double//slash",
		"semicolon;rm",
		"tilde~name",
		"caret^name",
		"colon:name",
		"quote\"name",
	}
	for _, name := range invalid {
		if ValidBranchName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestGithubSlug(t *testing.T) {
	cases := []struct {
		remote string
		want   string
		ok     bool
	}{
		{"git@github.com:owner/repo.git", "owner/repo", true},
		{"git@github.com:owner/repo", "owner/repo", true},
		{"https://github.com/owner/repo.git", "owner/repo", true},
		{"https://github.com/owner/repo", "owner/repo", true},
		{"ssh://git@github.com/owner/repo.git", "owner/repo", true},
		{"git@gitlab.com:owner/repo.git", "", false},
		{"https://example.com/owner/repo.git", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		got, ok := GithubSlug(c.remote)
		if ok != c.ok || got != c.want {
			t.Errorf("GithubSlug(%q) = (%q, %v), want (%q, %v)", c.remote, got, ok, c.want, c.ok)
		}
	}
}
