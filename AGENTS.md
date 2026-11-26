# AGENTS.md

Rules for AI coding agents in this repository.

## Public repository

This repository is public. No file, commit message, branch name or comment may name an internal project, host, address, bucket, person or product of a user of these tools.

## Tests

- Write a test for each change in what a command reports.
- No wall-clock waits in tests: `time.Sleep` and `time.After` only inside a `synctest.Test` bubble. `lint/wallclock` enforces this.
- Every top-level `TestXxx` calls `t.Parallel()`, unless `testdata/serial-tests.txt` lists it with a reason.
- Run Go tests with `-race`. A package is at most 10 s unless `testdata/test-time-budget.txt` says more.

## Comments

Write no comment by default. A comment states why: a constraint, a hazard, or a rejected alternative.

## Style

- Use standard Go style. Run `gofmt` and `go vet`.
- Use ASD-STE100 Simplified Technical English for prose.

## Commits

- Use Conventional Commit subjects: `type(scope): description`, lowercase, no final period.
- A commit body gives the problem, the design, the semantic change and the verification. Hard-wrap it at 76 columns.
- Add no attribution footer.

## Checks

ward lands a branch onto `main` only after it runs the checks of `.ward/checks.json`: `static` and `budget`. The runner runs the checks of a repository one after another. The check `budget` runs every test with `-race`, so it covers a plain `go test`, and it runs `testtime` with `-time-allowance 1.15`, because the land runners are slower than the reference environment. The counts stay strict.

```bash
go build ./... && go vet ./... && go run ./cmd/lint ./... && test -z "$(gofmt -l .)"
f=$(mktemp) && GORACE=atexit_sleep_ms=0 go test -race -count=1 -p 1 -json ./... > "$f"; a=$?; go run ./cmd/testtime -title Unit -job go -time-allowance 1.15 < "$f"; b=$?; rm -f "$f"; test $a -eq 0 && test $b -eq 0
```
