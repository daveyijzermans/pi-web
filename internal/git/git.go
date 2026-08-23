// Package git provides thin, read-mostly helpers around the local git CLI for
// the chat composer's branch indicator and PR button. All commands run with an
// explicit working directory and fixed argument lists (no shell), so session
// cwd values can never inject extra commands.
package git

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	// ErrNotRepo is returned when dir is not inside a git work tree.
	ErrNotRepo = errors.New("not a git repository")
	// ErrInvalidBranchName is returned when a requested branch name is empty
	// or contains characters git would reject.
	ErrInvalidBranchName = errors.New("invalid branch name")
	// ErrNoRemote is returned when no GitHub-style origin remote is configured.
	ErrNoRemote = errors.New("no github remote configured")
	// ErrDefaultBranch is returned when a rename targets the repository's
	// default branch, which we refuse to rename.
	ErrDefaultBranch = errors.New("refusing to rename the default branch")
)

// branchNamePattern is intentionally stricter than git's own check-ref-format:
// it covers the names humans actually type and rejects anything exotic.
var branchNamePattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// Info describes the git state surfaced in the composer footer.
type Info struct {
	IsRepo bool   `json:"isRepo"`
	Branch string `json:"branch"`
	// IsDefault marks the repository's default branch (no rename / no PR).
	IsDefault bool `json:"isDefault"`
	// HasChanges is true when the working tree is dirty or there are local
	// commits not yet pushed to the upstream — i.e. there's something to push.
	HasChanges bool `json:"hasChanges"`
	// Dirty is true when the working tree has unstaged or staged changes.
	Dirty bool `json:"dirty"`
	// Ahead is the number of local commits not yet pushed to the upstream branch.
	Ahead int `json:"ahead"`
	// Behind is the number of upstream commits not yet pulled locally.
	Behind int `json:"behind"`
	// Modified/Added/Deleted count working-tree changes by kind (for the footer badges).
	Modified int `json:"modified"`
	Added    int `json:"added"` // new / untracked files
	Deleted  int `json:"deleted"`
	// PRCreateURL is the GitHub "open a pull request" URL for this branch.
	PRCreateURL string `json:"prCreateUrl"`
	// PRURL is set when an OPEN pull request already exists for this branch,
	// in which case the UI offers "View PR" instead of "Create PR".
	PRURL string `json:"prUrl"`
}

// IssueInfo describes a single open GitHub issue.
type IssueInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

// PRInfo describes a single open GitHub pull request.
type PRInfo struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	URL    string `json:"url"`
}

func run(dir string, args ...string) (string, error) {
	// Bound every git invocation: a command that blocks (huge untracked tree,
	// network filesystem, a stuck index lock) must not hang the HTTP handler.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	// Trim only trailing newlines, not leading whitespace: `git status
	// --porcelain` encodes the staged-status column as a leading space (" M path"),
	// and stripping it would shift the path parse by one character. Single-value
	// outputs (branch names, counts, URLs) carry no leading whitespace, so they are
	// unaffected.
	return strings.TrimRight(string(out), "\r\n"), nil
}

// CurrentBranch returns the checked-out branch name for dir.
func CurrentBranch(dir string) (string, error) {
	if dir == "" {
		return "", ErrNotRepo
	}
	branch, err := run(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", ErrNotRepo
	}
	if branch == "" || branch == "HEAD" {
		// Detached HEAD or empty repo: no editable branch.
		return "", ErrNotRepo
	}
	return branch, nil
}

// Describe gathers the branch and a best-effort GitHub PR URL for dir. A
// non-repo directory yields Info{IsRepo: false} with a nil error so callers can
// simply hide the footer.
func Describe(dir string) (Info, error) {
	branch, err := CurrentBranch(dir)
	if err != nil {
		return Info{IsRepo: false}, nil
	}
	info := Info{IsRepo: true, Branch: branch}
	info.Dirty = isDirty(dir)
	info.Modified, info.Added, info.Deleted = dirtyCounts(dir)
	info.Ahead, info.Behind = aheadBehind(dir)
	info.HasChanges = info.Dirty || info.Ahead > 0
	if def := DefaultBranch(dir); def != "" && def == branch {
		info.IsDefault = true
	}
	if url, err := pullRequestURL(dir, branch); err == nil {
		info.PRCreateURL = url
	}
	// Only feature branches can have a PR against the default branch.
	if !info.IsDefault {
		info.PRURL = existingOpenPRURL(dir)
	}
	return info, nil
}

// HasLocalChanges reports whether there is something to commit or push: either
// a dirty working tree, or local commits ahead of the upstream branch.
func HasLocalChanges(dir string) bool {
	if isDirty(dir) {
		return true
	}
	ahead, _ := aheadBehind(dir)
	return ahead > 0
}

// isDirty reports whether the working tree has unstaged or staged changes.
func isDirty(dir string) bool {
	if out, err := run(dir, "status", "--porcelain"); err == nil {
		return out != ""
	}
	return false
}

// dirtyCounts tallies changed files by kind from `git status --porcelain` for
// the footer badges. Untracked files are folded into the "added"/new count
// (the footer shows N/M/D); the file tree gets untracked separately via
// Summarize on the /api/files/git-status endpoint.
func dirtyCounts(dir string) (modified, added, deleted int) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil || out == "" {
		return 0, 0, 0
	}
	s := Summarize(parsePorcelain(out))
	return s.Modified, s.Added + s.Untracked, s.Deleted
}

// aheadBehind returns the number of local commits ahead of upstream and
// upstream commits behind (not yet pulled). Returns (0, 0) if no upstream
// is configured or the command fails.
func aheadBehind(dir string) (ahead, behind int) {
	if out, err := run(dir, "rev-list", "--count", "@{upstream}..HEAD"); err == nil && out != "" {
		fmt.Sscanf(out, "%d", &ahead)
	}
	if out, err := run(dir, "rev-list", "--count", "HEAD..@{upstream}"); err == nil && out != "" {
		fmt.Sscanf(out, "%d", &behind)
	}
	return ahead, behind
}

// diffRun runs a git diff-style command and returns its stdout verbatim (no
// trimming, since patch whitespace is significant). git exits 1 to signal
// "differences found", which is not an error for us; any higher exit code or a
// failure to start the process is.
func diffRun(dir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return string(out), nil
		}
		return "", err
	}
	return string(out), nil
}

const (
	// maxDiffBytes bounds the combined patch so a repo with a huge working tree
	// can't produce a multi-megabyte payload that stalls the request or the
	// browser's diff renderer.
	maxDiffBytes = 2 << 20 // 2 MiB
	// maxUntrackedFileBytes skips reading the contents of very large untracked
	// files (they are shown as a placeholder instead).
	maxUntrackedFileBytes = 256 << 10 // 256 KiB
)

// WorkingTreeDiff returns a single unified patch covering every uncommitted
// change in dir: modifications to tracked files (staged and unstaged, compared
// against HEAD) followed by untracked files rendered as all-additions. It
// returns ErrNotRepo when dir is not a git work tree.
//
// Untracked files are synthesized in-process (one file read each) rather than
// shelling out to `git diff --no-index` per file, which previously spawned one
// subprocess per untracked file and could take tens of seconds in trees with
// thousands of untracked files. The combined output is capped at maxDiffBytes.
func WorkingTreeDiff(dir string) (string, error) {
	if dir == "" {
		return "", ErrNotRepo
	}
	if _, err := run(dir, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "", ErrNotRepo
	}
	var b strings.Builder
	// Tracked changes vs HEAD. In a repo with no commits yet HEAD is absent and
	// this fails harmlessly — the untracked pass below still surfaces new files.
	if tracked, err := diffRun(dir, "-c", "core.quotepath=false", "diff", "HEAD"); err == nil {
		b.WriteString(tracked)
	}
	if b.Len() >= maxDiffBytes {
		return b.String(), nil
	}
	others, err := run(dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err == nil && others != "" {
		for _, f := range strings.Split(others, "\x00") {
			if f == "" {
				continue
			}
			if b.Len() >= maxDiffBytes {
				break
			}
			appendUntrackedPatch(&b, dir, f)
		}
	}
	return b.String(), nil
}

// appendUntrackedPatch writes a synthetic "new file" patch for an untracked
// file. Directories, symlinks, and unreadable entries are skipped.
func appendUntrackedPatch(b *strings.Builder, dir, rel string) {
	full := filepath.Join(dir, rel)
	info, err := os.Lstat(full)
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	fmt.Fprintf(b, "diff --git a/%s b/%s\nnew file mode 100644\n", rel, rel)
	if info.Size() > maxUntrackedFileBytes {
		fmt.Fprintf(b, "--- /dev/null\n+++ b/%s\n@@ -0,0 +1 @@\n+[file too large to display]\n", rel)
		return
	}
	data, err := os.ReadFile(full)
	if err != nil {
		return
	}
	if isBinary(data) {
		fmt.Fprintf(b, "Binary files /dev/null and b/%s differ\n", rel)
		return
	}
	fmt.Fprintf(b, "--- /dev/null\n+++ b/%s\n", rel)
	if len(data) == 0 {
		return
	}
	s := string(data)
	noTrailingNewline := !strings.HasSuffix(s, "\n")
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	fmt.Fprintf(b, "@@ -0,0 +1,%d @@\n", len(lines))
	for i, line := range lines {
		b.WriteString("+")
		b.WriteString(line)
		b.WriteString("\n")
		if noTrailingNewline && i == len(lines)-1 {
			b.WriteString("\\ No newline at end of file\n")
		}
	}
}

// isBinary reports whether data looks binary (contains a NUL byte in its head),
// matching git's own heuristic closely enough for display purposes.
func isBinary(data []byte) bool {
	head := data
	if len(head) > 8000 {
		head = head[:8000]
	}
	return bytes.IndexByte(head, 0) >= 0
}

// existingOpenPRURL returns the URL of an OPEN pull request for the current
// branch, using the gh CLI when available. It is best-effort: a missing/
// unauthenticated gh, no PR, or a closed/merged PR all yield "".
func existingOpenPRURL(dir string) string {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gh, "pr", "view", "--json", "url,state")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var pr struct {
		URL   string `json:"url"`
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &pr); err != nil {
		return ""
	}
	if strings.EqualFold(pr.State, "OPEN") {
		return pr.URL
	}
	return ""
}

// OpenIssues returns the list of open GitHub issues for the repository in dir,
// using the gh CLI when available. It is best-effort: a missing/unauthenticated
// gh, non-repo dir, or network error all yield nil.
func OpenIssues(dir string) []IssueInfo {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gh, "issue", "list", "--state", "open", "--json", "number,title,url")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var issues []IssueInfo
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil
	}
	return issues
}

// OpenPRs returns the list of open GitHub pull requests for the repository in
// dir, using the gh CLI when available. It is best-effort: a missing/
// unauthenticated gh, non-repo dir, or network error all yield nil.
func OpenPRs(dir string) []PRInfo {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gh, "pr", "list", "--state", "open", "--json", "number,title,url")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var prs []PRInfo
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil
	}
	return prs
}

// RepoDescription returns the GitHub repository description for the repository
// in dir, using the gh CLI when available. It is best-effort: a missing/
// unauthenticated gh, non-repo dir, or network error all yield "".
func RepoDescription(dir string) string {
	gh, err := exec.LookPath("gh")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, gh, "repo", "view", "--json", "description")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	var rv struct {
		Description string `json:"description"`
	}
	if err := json.Unmarshal(out, &rv); err != nil {
		return ""
	}
	return rv.Description
}

// DefaultBranch reports the repository's default branch. It prefers the
// remote's published HEAD (origin/HEAD) and falls back to a local main/master
// when that isn't configured. Returns "" when it can't be determined.
func DefaultBranch(dir string) string {
	if out, err := run(dir, "symbolic-ref", "--short", "refs/remotes/origin/HEAD"); err == nil && out != "" {
		return strings.TrimPrefix(out, "origin/")
	}
	for _, candidate := range []string{"main", "master"} {
		if _, err := run(dir, "rev-parse", "--verify", "--quiet", "refs/heads/"+candidate); err == nil {
			return candidate
		}
	}
	return ""
}

// RenameBranch renames the currently checked-out branch to name via
// `git branch -m`, validating the name first.
func RenameBranch(dir, name string) (string, error) {
	name = strings.TrimSpace(name)
	if !ValidBranchName(name) {
		return "", ErrInvalidBranchName
	}
	branch, err := CurrentBranch(dir)
	if err != nil {
		return "", err
	}
	if def := DefaultBranch(dir); def != "" && def == branch {
		return "", ErrDefaultBranch
	}
	cmd := exec.Command("git", "branch", "-m", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("%s", msg)
	}
	return name, nil
}

// ValidBranchName reports whether name is safe to pass to git branch -m.
func ValidBranchName(name string) bool {
	if name == "" || len(name) > 255 {
		return false
	}
	if strings.HasPrefix(name, "-") || strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") {
		return false
	}
	if strings.Contains(name, "..") || strings.Contains(name, "//") {
		return false
	}
	return branchNamePattern.MatchString(name)
}

// pullRequestURL turns the origin remote + branch into a GitHub "open a pull
// request" URL. Supports both SSH (git@github.com:owner/repo.git) and HTTPS
// remotes. Returns ErrNoRemote for non-GitHub or missing remotes.
func pullRequestURL(dir, branch string) (string, error) {
	remote, err := run(dir, "remote", "get-url", "origin")
	if err != nil || remote == "" {
		return "", ErrNoRemote
	}
	slug, ok := GithubSlug(remote)
	if !ok {
		return "", ErrNoRemote
	}
	return fmt.Sprintf("https://github.com/%s/pull/new/%s", slug, branch), nil
}

// Summary counts working-tree changes by kind, untracked separate from added.
type Summary struct {
	Modified  int `json:"modified"`
	Added     int `json:"added"`
	Deleted   int `json:"deleted"`
	Untracked int `json:"untracked"`
}

// Summarize classifies each DirtyFile by its status code.
// "??" -> Untracked, contains 'A' -> Added, contains 'D' -> Deleted,
// otherwise (M, R, C, etc.) -> Modified.
func Summarize(files []DirtyFile) Summary {
	var s Summary
	for _, f := range files {
		switch {
		case f.Status == "??":
			s.Untracked++
		case strings.ContainsRune(f.Status, 'A'):
			s.Added++
		case strings.ContainsRune(f.Status, 'D'):
			s.Deleted++
		default:
			s.Modified++
		}
	}
	return s
}

// DirtyFile is one changed entry from `git status --porcelain`.
type DirtyFile struct {
	Status string `json:"status"` // trimmed XY code, e.g. "M", "A", "D", "R", "??"
	Path   string `json:"path"`   // path with the status prefix stripped; rename dest
}

// parsePorcelain parses git status --porcelain output into DirtyFile entries.
// Porcelain v1: 2 status chars (XY) + 1 space + path. Renames use " -> " or tabs.
func parsePorcelain(out string) []DirtyFile {
	lines := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
	result := make([]DirtyFile, 0, len(lines))
	for _, line := range lines {
		if len(line) < 3 {
			result = append(result, DirtyFile{Path: strings.TrimSpace(line)})
			continue
		}
		status := strings.TrimSpace(line[:2])
		rest := strings.TrimSpace(line[3:])
		// For renames/copies, extract the destination after " -> ".
		if idx := strings.Index(rest, " -> "); idx >= 0 {
			rest = strings.TrimSpace(rest[idx+4:])
		}
		result = append(result, DirtyFile{Status: status, Path: rest})
	}
	return result
}

// DirtyFiles returns the list of changed files from `git status --porcelain`.
// Returns nil if the working tree is clean.
func DirtyFiles(dir string) ([]DirtyFile, error) {
	out, err := run(dir, "status", "--porcelain")
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return parsePorcelain(out), nil
}

// GithubSlug extracts "owner/repo" from a github remote URL, or returns false.
func GithubSlug(remote string) (string, bool) {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")

	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		return strings.TrimPrefix(remote, "git@github.com:"), true
	case strings.HasPrefix(remote, "ssh://git@github.com/"):
		return strings.TrimPrefix(remote, "ssh://git@github.com/"), true
	case strings.HasPrefix(remote, "https://github.com/"):
		return strings.TrimPrefix(remote, "https://github.com/"), true
	case strings.HasPrefix(remote, "http://github.com/"):
		return strings.TrimPrefix(remote, "http://github.com/"), true
	}
	return "", false
}
