package testcount

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLineDividesByRunsAndParses(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		runs int
		want int
	}{{1, 5}, {2, 3}, {5, 1}, {0, 5}} {
		got, ok := Parse("ok\n" + Line(map[string]int{Servers: 5}, c.runs))
		if !ok || got[Servers] != c.want {
			t.Errorf("runs %d: servers = %d (ok %v), want %d", c.runs, got[Servers], ok, c.want)
		}
	}
	if _, ok := Parse("PASS\n"); ok {
		t.Error("Parse found a count line in output without one")
	}
}

const childEnv = "TESTCOUNT_AT_EXIT_CHILD"

func TestMain(m *testing.M) { os.Exit(Run(m)) }

func TestAtExitRunsAfterTestsBeforeTheCountLine(t *testing.T) {
	t.Parallel()
	if os.Getenv(childEnv) != "" {
		if !AtExit(func() { fmt.Println("at-exit-ran") }) {
			t.Fatal("AtExit refused under Run")
		}
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestAtExitRunsAfterTestsBeforeTheCountLine$")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child failed: %v\n%s", err, out)
	}
	ran, line := strings.Index(string(out), "at-exit-ran"), strings.Index(string(out), Prefix)
	if ran < 0 || line < 0 || ran > line {
		t.Fatalf("at-exit-ran at %d, count line at %d, want the callback before the line:\n%s", ran, line, out)
	}
}
