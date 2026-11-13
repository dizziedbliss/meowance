# meowance

meow + attendance. A web app for MCE (Malnad College of Engineering) students: log in with your
own USN + birthday, and it pulls your attendance from the college's Contineo portal and tells you
how many classes you must attend (or can still skip) to stay out of the fine zone. Personal
project, not public yet.

Full plan & confirmed portal facts: `~/.claude/plans/meowance-attendance-portal.md`.

## Commands

- `go test ./...` / `go vet ./...` — Go tests and checks
- `go run ./cmd/devserver` — local API on :8787 against the **real** portal
- `go run ./cmd/devserver -fixture pkg/portal/testdata/dashboard.html` — local API serving the
  scrubbed fixtures (dashboard + `timetable.html` beside it) for any login (birthday `01012000`
  simulates a wrong login). `-addr` changes the port.
- `cd web && npm run dev` — Vite on **:5180** (all interfaces, so a Windows browser reaches it in
  WSL), proxies `/api` to :8787, or to `$MEOWANCE_API`. `npm run preview` uses :5181.
- `go run` doesn't reload: restart the dev server after Go changes. Stopping a `go run` task can
  leave the compiled `devserver` running; check the port with `ss -ltnp` and kill that pid.
- `cd web && npm test` / `npm run typecheck` / `npm run build`
- `go run ./cmd/demo [dashboard.html]` — print the bunk table for a saved dashboard

## Architecture

- **`pkg/portal`**: `client.go` logs into Contineo (GET login page → scrape Joomla CSRF token →
  POST `username` + `passwd`=DOB `YYYY-MM-DD` → follow 303s) with a fresh cookie jar per call, one
  retry on network failure only; then, only if asked (`Extras`), this week's timetable and the
  calendar of events, in parallel in the same session, with the dashboard as Referer (best effort:
  nil on failure). `Client.Logf` logs
  one line per extra page (status/size/parsed), never credentials or URLs. `dashboard.go` parses the
  dashboard, including each course's CIE parts and total; `timetable.go` the week (the faculty column
  is never parsed); `calendar.go` the semester dates, holidays, exam days and events. `proxy.go`: with
  `OXYLABS_USERNAME`/`OXYLABS_PASSWORD` set, requests go through a sticky residential proxy session
  (the portal blocks datacenter IPs). Each login attempt is pinned to one session ID via its
  context; a 403 retires only that ID and retries. The handler gives a login 50s in all
  (`fetchBudget`), under Vercel's 60s cap. The login POST is sent
  exactly once per attempt (a test counts it).
- **`pkg/attend`**: pure bunk math over attended/conducted: `Recover` (classes in a row to reach a
  target), `CanBunk` (classes you can miss and stay at/above it). The math lives only here.
- **`pkg/api`**: `POST /api/attendance` `{usn, dob: DDMMYYYY, extras?: ["timetable","calendar"]}`
  → `{student, courses, timetable?, calendar?}` with `need75`, `need85`, `skip85` and
  `cie`/`cieTotal`/`cieMax` per course. An extra not asked for is left out; asked but failed is
  `null`. The dashboard always comes back (the login lands on it). Validates MCE USNs (`4MC…`) and real dates. Error codes:
  `bad_input` 400, `bad_login` 401, `portal_down`/`portal_changed`/`portal_blocked` (portal 403s our
  IP) 502. `Cache-Control: no-store`.
- **`api/attendance.go`**: the Vercel Go function; just wraps `pkg/api` with the real client.
- **`cmd/devserver`**: same handler locally, optional fixture mode.
- **`web/`**: Vite + React 19 + Tailwind v4. `lib/verdict.ts` turns the API numbers into the copy
  (zones, "attend N more classes", lives); `lib/week.ts` tags each timetable class from its subject's
  zone (bunkable / show up / don't you dare); `lib/scores.ts` the CIE verdicts; `lib/events.ts`
  working days left, upcoming/past split, event tags; `lib/dates.ts` the portal's date formats; `lib/cache.ts` which extras are stale (the
  timetable after the week changes, the calendar after 7 days, either after a failed fetch) and
  merges replies into the cache. Login and sync fetch the dashboard only; opening a tab fetches its
  page when stale; sync on a tab refreshes that page too. `lib/vault.ts` keeps creds encrypted with a non-extractable
  WebCrypto key (IndexedDB) and the last dashboard in localStorage; logout wipes both.

## Conventions

- **MCE rules:** 85%+ safe ("purring"); 75–85 fine zone (pays a fine unless back to 85); under 75 dead
  zone (teachers cap at 75). Verdicts count **classes**, never days, and never assume how many
  classes are left in the sem.
- **UI:** dark only, sharp corners, no boxed cards (rows split by hairlines), one loud element per
  screen, bottom nav is an overlay with its own solid background (it mustn't blend in). No endless
  scroll: secondary content goes in sub-tabs or collapsibles. Tests are "cie", never "se". Copy is lowercase, dry gen-z; cat puns welcome
  (pawsent, lives, purring). Max's own logos in `web/public/brand/`; never the college crest, never
  student or lecturer photos. Dept = the 2 letters in the USN.
- Credentials are never stored server-side or logged. Test fixtures are PII-scrubbed (USN
  `4MC00AA000`, name `STUDENT`, every lecturer name `FACULTY`, all profile photos); never commit real
  student data or HARs. Scrub by markup position, then grep for real names.
- Go: stdlib-first. Web: exact-pinned deps, `npm ci`.

## Not built yet

- Timetable only shows the current week (no previous/next week).
- Bunk advice uses the portal's own "still to go" count per course (`lib/history.ts`), phrased as
  "you can bunk N" with the caveat that the faculty must take every class left, because the
  portal's counts are often stale. CIE pass rules (3 cie + 2 activities, 12 + 8 to pass) aren't
  built: the juniors' scheme differs and is unknown.
- The portal 403s Vercel's Mumbai IPs directly (seen 2026-11-13 on a preview without the proxy);
  through the Oxylabs proxy, prod reaches it (a fake USN gets 401 `bad_login` in ~3s). A real
  student login through prod still needs Max's own USN to confirm the full dashboard. The Oxylabs
  vars are set for Production only, so previews can't test the proxy.

## Deploy (Vercel)

- **Live:** https://meowance.vercel.app (project `dizziedbliss/meowance`, first deployed 2026-11-09).
  Firewall rule "attendance api limit": 60 requests/min per IP on `/api/attendance`, deny when
  exceeded (college Wi-Fi puts everyone behind one IP, so keep it generous). Previews are behind
  Vercel Authentication; use `vercel curl <url>` to test them.
- **Deploys only happen from the CLI**: `git.deploymentEnabled: false`, because Vercel aimed the first
  branch push of the new project at production. Preview: `vercel deploy`. Production: `vercel deploy
  --prod`, only with Max's OK. `.vercelignore` keeps `.env*`, `.vercel`, deps, builds and HARs out of uploads.
- Shared Go code lives in `pkg/`, not `internal/`: Vercel compiles `/api` as its own package
  (`handler/api`), and Go forbids it importing another module's `internal/`.
- `vercel.json`: functions run in `bom1` (Mumbai, near the portal), 60s max; security headers on every
  path (CSP with `script-src 'self'`, so the build must never inline a script; `frame-ancestors 'none'`).
- Brute-force guard: per 15 min, 5 wrong birthdays per (USN, client IP) and 20 per USN overall →
  429 `too_many_tries`, portal not contacted. Attempts are reserved before the portal call (no
  parallel bypass), released on outages, cleared on a correct login. Never evicts a blocked key.
  (`pkg/api/guard.go`, in-memory per instance). Pair it with a Vercel firewall per-IP rate limit
  on `/api/attendance`.

## Known issues

- The guard is per warm instance, not global; the firewall rule is the cross-instance limit.
- When its table is full of live counts (100k keys in 15 min) the guard fails closed: new USNs get
  429 for up to a minute. Deliberate: fail-open was a bypass. Full sweeps are throttled to one per 10s.
- Login is only as safe as MCE's (USN + birthday).
