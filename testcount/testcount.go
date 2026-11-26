// Package testcount counts the expensive operations that test helpers start and prints the counts of a package at
// the end of its test run, for testtime to check against testdata/test-time-budget.txt.
package testcount

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// The names of the counts.
const (
	Servers   = "servers"
	Schemas   = "schemas"
	Processes = "processes"
)

// Prefix starts the one output line that carries the counts of a package.
const Prefix = "testcount "

var (
	mu     sync.Mutex
	counts = map[string]int{Servers: 0, Schemas: 0, Processes: 0}
	hooked atomic.Bool
	exits  []func()
)

// AtExit registers f to run after the tests of the package, before Run prints the counts. It reports false, and
// does not register f, when the package does not call Run from its TestMain.
func AtExit(f func()) bool {
	mu.Lock()
	defer mu.Unlock()
	if !hooked.Load() {
		return false
	}
	exits = append(exits, f)
	return true
}

// Add counts one operation. When the helper that calls Add belongs to the module under test, and the package does
// not call Run from its TestMain, Add fails the test, because the count would be lost. A helper of another module
// counts only.
func Add(t testing.TB, name string) {
	t.Helper()
	if !hooked.Load() && inModule() {
		t.Fatalf("the package does not call testcount.Run from TestMain, so it cannot count %s", name)
	}
	mu.Lock()
	counts[name]++
	mu.Unlock()
}

// Command counts a started process and returns exec.Command(name, args...).
func Command(t testing.TB, name string, args ...string) *exec.Cmd {
	t.Helper()
	Add(t, Processes)
	return exec.Command(name, args...)
}

// Run runs the tests of m, prints the count line and returns the exit code. Use it as os.Exit(testcount.Run(m)).
func Run(m *testing.M) int {
	hooked.Store(true)
	code := m.Run()
	mu.Lock()
	fns := exits
	mu.Unlock()
	for _, f := range fns {
		f()
	}
	runs := 1
	if f := flag.Lookup("test.count"); f != nil {
		_, _ = fmt.Sscan(f.Value.String(), &runs)
	}
	mu.Lock()
	line := Line(counts, runs)
	mu.Unlock()
	fmt.Print("\n" + line)
	return code
}

// Line renders the count line for the counts of runs runs of the tests. Each count is divided by runs, rounded up.
func Line(counts map[string]int, runs int) string {
	runs = max(runs, 1)
	one := make(map[string]int, len(counts))
	for k, v := range counts {
		one[k] = (v + runs - 1) / runs
	}
	data, _ := json.Marshal(one)
	return Prefix + string(data) + "\n"
}

// Parse returns the counts of a test output that holds a count line. The bool is false when it holds none.
func Parse(output string) (map[string]int, bool) {
	i := strings.Index(output, Prefix)
	if i < 0 {
		return nil, false
	}
	rest, _, _ := strings.Cut(output[i+len(Prefix):], "\n")
	var out map[string]int
	if err := json.Unmarshal([]byte(rest), &out); err != nil {
		return nil, false
	}
	return out, true
}

// inModule reports whether the first caller outside this package belongs to the main module of the test binary.
func inModule() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Path == "" || os.Getenv("TESTCOUNT_OFF") != "" {
		return false
	}
	pcs := make([]uintptr, 16)
	frames := runtime.CallersFrames(pcs[:runtime.Callers(2, pcs)])
	self := reflect.TypeFor[hookFlag]().PkgPath()
	for {
		f, more := frames.Next()
		if pkg := funcPackage(f.Function); pkg != self && pkg != "" {
			return pkg == info.Main.Path || strings.HasPrefix(pkg, info.Main.Path+"/")
		}
		if !more {
			return false
		}
	}
}

type hookFlag struct{}

// funcPackage returns the package path of a function name such as example.com/a/b.F or example.com/a/b.(*T).M.
func funcPackage(fn string) string {
	slash := strings.LastIndex(fn, "/")
	dot := strings.Index(fn[slash+1:], ".")
	if dot < 0 {
		return ""
	}
	return fn[:slash+1+dot]
}
