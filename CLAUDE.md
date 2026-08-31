@AGENTS.md

# Fork-specific rules (Windows dev box)

- Do not write, edit, or build code until the user says to proceed. Investigate, propose a plan, wait for approval.
- Pre-flight before any source edit: working tree clean (`git status`), on a `type/issueN-description` branch (never `main`), and an open issue covers the work. If any check fails, stop and tell the user.
- Commits/PRs/issues: follow `docs/dev/workflow.md` — labels mandatory, conventional commits (`type(scope): desc` + `Closes #N`), one issue one PR, never close an issue before its PR merges. Run `cd web && npm run format` before every PR; all 3 CI checks (`gh pr checks`) must pass before merging.
- Interact only with the fork (`Hrt-Htk/pi-web`), never upstream (`ygncode/pi-web`).

## Testing

Test-first for behavior changes (backend handlers, worker/session logic, Svelte components, bug fixes): write the failing test, make it pass, refactor. Docs/chore/CI changes and logic-free UI tweaks are exempt. Pick the cheapest proving layer — unit (`make test`) → component (vitest + testing-library) → backend integration (`internal/integration/`) → Playwright (`make e2e`, not in `check`). UI changes additionally get a manual look in the browser against the test server before committing.

## Environment

- Go: `/c/Users/HTK/go/bin/go.exe` (add to PATH if not found: `export PATH="$PATH:/c/Users/HTK/go/bin"`)
- Always build with `-o pi-web.exe` — bare `go build` output has no extension and won't run on Windows.
- Node: `/c/nvm4w/nodejs/` (npm at `C:/nvm4w/nodejs/npm.cmd`)
- make: GNU Make 4.4.1 via winget (`C:/Users/HTK/AppData/Local/Microsoft/WinGet/Packages/ezwinports.make_.../bin`). Its recipes run via `sh`, which can't resolve `npm.cmd`; when `make build` fails at the npm step, run directly: `cd web && /c/nvm4w/nodejs/npm.cmd run build` then `go build -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty)" -o pi-web.exe ./cmd/pi-web`. Never ship a `go build` that skipped the frontend build (`//go:embed`).

## Servers

- Prod: `h:\software\pi-web-prod\pi-web.exe`, port 31415, auth via `PI_WEB_TOKEN` user env var. Never stop/kill/restart it during dev or test work, and never by process name (`Stop-Process -Name pi-web` / `taskkill /IM` kill every instance). The only sanctioned stop/replace is `scripts/deploy.ps1`; `h:\software\pi-web-prod\start-prod.ps1` starts it without redeploying (e.g. after a reboot).
- Test server: port 31416 via `pwsh -ExecutionPolicy Bypass -File scripts/start-test-server.ps1` (records PID to `.tmp/test-server.pid`); stop with `scripts/stop-test-server.ps1` (kills only that PID). `-dev` disables auth, binds loopback, skips Tailscale Serve — but it serves the real agent dir, so chat/rename on 31416 mutates real session data. For scratch chat tests, create a throwaway session via `POST /api/new-session` and delete its `.jsonl` afterward. Never run tests against 31415.
- E2E spawns its own server on a free port (`e2e/lib/server.ts`) — just `cd e2e && npx playwright test <spec>`.

## Deploying to prod

Deploy is a deliberate action, separate from dev/test work:

1. Build (frontend build, then `go build -o pi-web.exe`).
2. `pwsh -ExecutionPolicy Bypass -File scripts/deploy.ps1` — stops prod by recorded PID, copies the binary, restarts on 31415, records the new PID.
3. Verify `http://localhost:31415/` returns 401 (auth on) and the version matches the build.
