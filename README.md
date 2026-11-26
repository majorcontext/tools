# tools

Go test and lint tools that the majorcontext repositories share.

| Path | What it does |
|---|---|
| `cmd/lint` | Runs the analyzers `wallclock` and `paralleltests` over packages. |
| `cmd/testtime` | Reads `go test -json` output and fails a package or test that passes its time or count budget in `testdata/test-time-budget.txt`. |
| `cmd/leaklint` | Fails when a tracked file or path holds a term of the `LEAK_DENYLIST` environment variable. |
| `lint/wallclock` | Refuses `time.Sleep` and `time.After` outside a `synctest` bubble. |
| `lint/paralleltests` | Requires `t.Parallel()` in each top-level test, except the tests that `testdata/serial-tests.txt` lists. |
| `testcount` | Counts the servers, schemas and processes that test helpers start, for `testtime`. |
| `seams` | Checks the provider seams table of `docs/architecture.md` against the code. |

## Use

Add the commands as tool dependencies of a module, and run them with `go tool`:

```sh
go get -tool github.com/majorcontext/tools/cmd/lint@latest
go get -tool github.com/majorcontext/tools/cmd/testtime@latest
go get -tool github.com/majorcontext/tools/cmd/leaklint@latest

go tool lint ./...
go test -race -count=1 -p 1 -json ./... | go tool testtime
```

Each command reads its files, such as `testdata/serial-tests.txt` and `testdata/test-time-budget.txt`, from the working directory, so run it from the module root. A module calls `testcount.Run` from the `TestMain` of each package whose helpers count, and `seams.CheckRepository(t, root)` from one test.

## License

MIT
