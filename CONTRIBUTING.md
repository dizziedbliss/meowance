# contributing to meowance

contributions are open!! fix a bug, add a thing, open a pr, n max reviews it.

## run it locally

you need Go and Node.

```bash
go run ./cmd/devserver -fixture pkg/portal/testdata/dashboard.html   # api on :8787, mock data, any login
cd web && npm ci && npm run dev                                       # app on :5180
```

fixture mode works for any usn; birthday `01012000` acts like a wrong login. `CLAUDE.md` maps out the
whole codebase.

## before u open a pr

```bash
gofmt -l . && go vet ./... && go test ./...
cd web && npm run typecheck && npm test && npm run build
```

CI runs the same on every pr.

## house rules

- **no real student data, ever.** test fixtures are mock data: real page layouts, every value
  invented (USN `4MC00AA000`, `STUDENT`, `FACULTY`). no real HARs, screenshots or marks.
- credentials are never stored server-side or logged.
- the bunk math lives in `pkg/attend`; predictions are only ever a small extra line, never the main
  "attend N more" verdict.
- ui: dark only, sharp corners, no boxed cards, lowercase copy written the way a student talks.
- one thing per pr, with a line on how u checked it.
