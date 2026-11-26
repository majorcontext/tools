package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParentWhoseSubtestsAllSkipIsNotALeaf(t *testing.T) {
	t.Parallel()
	stream := strings.Join([]string{
		`{"Action":"skip","Package":"example.com/p","Test":"TestA/one","Elapsed":0}`,
		`{"Action":"skip","Package":"example.com/p","Test":"TestA/two","Elapsed":0}`,
		`{"Action":"pass","Package":"example.com/p","Test":"TestA","Elapsed":99}`,
		`{"Action":"pass","Package":"example.com/p","Test":"TestB","Elapsed":99}`,
	}, "\n")
	rep := consume(strings.NewReader(stream), io.Discard)
	b := &budget{leaf: 10, pkgBase: 10, pkgs: map[string]float64{}}
	over := rep.check(b, "")
	if len(over) != 1 || !strings.Contains(over[0], "TestB") {
		t.Fatalf("over = %q, want only TestB", over)
	}
}

func TestPackageFailurePrintsTheOutputOfARunningTest(t *testing.T) {
	t.Parallel()
	stream := strings.Join([]string{
		`{"Action":"output","Package":"example.com/p","Test":"TestHang","Output":"panic: test timed out\n"}`,
		`{"Action":"fail","Package":"example.com/p","Elapsed":600}`,
	}, "\n")
	var out strings.Builder
	rep := consume(strings.NewReader(stream), &out)
	if !rep.failed || !strings.Contains(out.String(), "panic: test timed out") {
		t.Fatalf("failed = %v, output = %q, want the held trace printed", rep.failed, out.String())
	}
}

const baseBudget = "leaf 10\npackage default 10\ncalibrate 0.1\n"

func parse(t *testing.T, text string) (*budget, error) {
	path := filepath.Join(t.TempDir(), "b.txt")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return readBudget(path)
}

func TestBudgetFileCounts(t *testing.T) {
	t.Parallel()
	b, err := parse(t, baseBudget+"count p servers 3\ncount default servers 1\ncount q processes 0\n")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		pkg, name string
		want      int
	}{{"p", "servers", 3}, {"other", "servers", 1}, {"q", "processes", 0}, {"p", "schemas", 0}} {
		if got := b.countLimit(c.pkg, c.name); got != c.want {
			t.Errorf("countLimit(%s, %s) = %d, want %d", c.pkg, c.name, got, c.want)
		}
	}
	for _, bad := range []string{"count p servers", "count p servers -1", "count p servers 1.5", "count p servers 1 2"} {
		if _, err := parse(t, baseBudget+bad+"\n"); err == nil {
			t.Errorf("%q parsed, want an error", bad)
		}
	}
	if _, err := parse(t, "leaf 10\npackage default 10\n"); err == nil {
		t.Error("a budget file without calibrate parsed")
	}
}

func TestCountsOverTheLimitFailThePackage(t *testing.T) {
	t.Parallel()
	stream := strings.Join([]string{
		`{"Action":"output","Package":"example.com/p","Output":"\n"}`,
		`{"Action":"output","Package":"example.com/p","Output":"testcount {\"processes\":4,\"servers\":2}\n"}`,
		`{"Action":"pass","Package":"example.com/p","Elapsed":1}`,
	}, "\n")
	var out strings.Builder
	rep := consume(strings.NewReader(stream), &out)
	b := &budget{leaf: 10, pkgBase: 10, pkgs: map[string]float64{}, counts: map[string]map[string]int{"default": {"servers": 2, "processes": 3}}}
	over := rep.check(b, "")
	if len(over) != 1 || !strings.Contains(over[0], "counted 4 processes, limit 3") || strings.Contains(out.String(), "testcount {") {
		t.Fatalf("over = %q, output = %q, want only processes over and the count line held back", over, out.String())
	}
}

func TestFactorNeverTightensAndScalesTimeLimits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct{ measured, reference, allowance, want float64 }{{0.5, 0.25, 1, 2}, {0.05, 0.1, 1, 1}, {0.1, 0.1, 1, 1}, {1, 0, 1, 1}, {0.5, 0.25, 1.5, 3}, {0.05, 0.1, 2, 2}} {
		if got := factorFor(c.measured, c.reference, c.allowance); got != c.want {
			t.Errorf("factorFor(%v, %v, %v) = %v, want %v", c.measured, c.reference, c.allowance, got, c.want)
		}
	}
	b := &budget{leaf: 10, pkgBase: 10, pkgs: map[string]float64{"p": 20}, factor: 2}
	if got := b.testLimit("p", "TestX"); got != 20 {
		t.Errorf("testLimit = %v, want 20", got)
	}
	if got := b.packageLimit("other", ""); got != 20 {
		t.Errorf("default packageLimit = %v, want 20", got)
	}
	if got := b.packageLimit("p", ""); got != 40 {
		t.Errorf("packageLimit = %v, want 40", got)
	}
}

func TestParallelismIsTestSecondsOverPackageSeconds(t *testing.T) {
	t.Parallel()
	stream := strings.Join([]string{
		`{"Action":"pass","Package":"example.com/p","Test":"TestA","Elapsed":3}`,
		`{"Action":"pass","Package":"example.com/p","Test":"TestB/one","Elapsed":3}`,
		`{"Action":"pass","Package":"example.com/p","Test":"TestB","Elapsed":9}`,
		`{"Action":"pass","Package":"example.com/p","Elapsed":2}`,
	}, "\n")
	got := consume(strings.NewReader(stream), io.Discard).parallelism()
	if len(got) != 1 || got[0].tests != 6 || got[0].ratio != 3 {
		t.Fatalf("parallelism = %+v, want one package with 6 test seconds and ratio 3", got)
	}
}
