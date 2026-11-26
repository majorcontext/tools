// Command testtime reads the output of go test -json on standard input, prints
// the readable test log, and fails when a test or a package is slower than the
// budget of testdata/test-time-budget.txt. Run it from the repository root.
// CI runs go test -p 1, so each package is timed alone.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/majorcontext/tools/internal/modpath"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/majorcontext/tools/testcount"
)

// modulePath is the path of the module in the working directory, which shortPackage strips from package paths.
var modulePath string

type event struct {
	Action  string
	Package string
	Test    string
	Output  string
	Elapsed float64
}

type result struct {
	pkg     string
	name    string
	elapsed float64
}

type report struct {
	tests    []result
	packages []result
	parents  map[string]bool
	counts   map[string]map[string]int
	note     string
	failed   bool
}

func main() {
	budgetFile := flag.String("budget", "testdata/test-time-budget.txt", "budget file")
	summaryFile := flag.String("summary", os.Getenv("GITHUB_STEP_SUMMARY"), "file that receives a Markdown summary")
	title := flag.String("title", "", "heading of the step summary")
	job := flag.String("job", "", "job name; a package budget PATH@JOB beats PATH")
	allowance := flag.Float64("time-allowance", 1, "factor of at least 1 that multiplies every time limit and no count, for a run on a sandbox slower than the reference")
	flag.Parse()
	mod, err := modpath.Read(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, "testtime: run it from the module root:", err)
		os.Exit(2)
	}
	modulePath = mod
	budget, err := readBudget(*budgetFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, "testtime:", err)
		os.Exit(1)
	}
	rep := consume(os.Stdin, os.Stdout)
	measured := calibrate()
	if *allowance < 1 {
		fmt.Fprintln(os.Stderr, "testtime: -time-allowance is at least 1")
		os.Exit(2)
	}
	budget.factor = factorFor(measured, budget.calibrate, *allowance)
	rep.note = fmt.Sprintf("Calibration: %.3f s here, %.3f s on the reference runner, allowance %.2f, factor %.2f.", measured, budget.calibrate, *allowance, budget.factor)
	fmt.Println("\ntesttime:", rep.note)
	over := rep.check(budget, *job)
	text := summary(*title, rep, over)
	fmt.Print("\n" + text)
	if path := *summaryFile; path != "" {
		if err := appendFile(path, text); err != nil {
			fmt.Fprintln(os.Stderr, "testtime:", err)
		}
	}
	for _, line := range over {
		fmt.Fprintln(os.Stderr, "testtime: over budget:", line)
	}
	if len(over) > 0 || rep.failed {
		os.Exit(1)
	}
}

// consume prints the log of the stream: package lines at once and the output
// of a test only when that test fails.
func consume(in io.Reader, out io.Writer) *report {
	rep := &report{parents: map[string]bool{}, counts: map[string]map[string]int{}}
	held := map[string][]string{}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		line := sc.Text()
		var ev event
		if err := json.Unmarshal([]byte(line), &ev); err != nil || ev.Action == "" {
			_, _ = fmt.Fprintln(out, line)
			continue
		}
		rep.markParent(ev)
		key := ev.Package + "\x00" + ev.Test
		switch ev.Action {
		case "output":
			if c, ok := testcount.Parse(ev.Output); ok && ev.Test == "" {
				rep.counts[shortPackage(ev.Package)] = c
			} else if ev.Test == "" {
				_, _ = fmt.Fprint(out, ev.Output)
			} else {
				held[key] = append(held[key], ev.Output)
			}
		case "pass", "fail":
			rep.record(ev)
			if ev.Action == "fail" {
				rep.failed = true
				for _, text := range held[key] {
					_, _ = fmt.Fprint(out, text)
				}
			}
			delete(held, key)
			if ev.Test == "" {
				flushPackage(out, held, ev.Package, ev.Action == "fail")
			}
		case "skip":
			delete(held, key)
		}
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "testtime:", err)
		rep.failed = true
	}
	return rep
}

func (r *report) record(ev event) {
	pkg := shortPackage(ev.Package)
	if ev.Test == "" {
		r.packages = append(r.packages, result{pkg: pkg, elapsed: ev.Elapsed})
		return
	}
	r.tests = append(r.tests, result{pkg: pkg, name: ev.Test, elapsed: ev.Elapsed})
}

// markParent records the parent of a nested test on any event, so a parent
// whose subtests all skip is still not a leaf.
func (r *report) markParent(ev event) {
	if i := strings.LastIndex(ev.Test, "/"); i >= 0 {
		r.parents[shortPackage(ev.Package)+"\x00"+ev.Test[:i]] = true
	}
}

func shortPackage(pkg string) string {
	pkg = strings.TrimPrefix(strings.TrimPrefix(pkg, modulePath), "/")
	if pkg == "" {
		return "."
	}
	return pkg
}

// check compares every leaf test and every package with its budget. A test
// with subtests is not a leaf: its time is the sum of the time of its parts.
func (r *report) check(b *budget, job string) []string {
	var over []string
	for _, t := range r.tests {
		if r.parents[t.pkg+"\x00"+t.name] {
			continue
		}
		if limit := b.testLimit(t.pkg, t.name); t.elapsed > limit {
			over = append(over, fmt.Sprintf("%s %s took %.1fs, budget %.1fs", t.pkg, t.name, t.elapsed, limit))
		}
	}
	for _, p := range r.packages {
		if limit := b.packageLimit(p.pkg, job); p.elapsed > limit {
			over = append(over, fmt.Sprintf("package %s took %.1fs, budget %.1fs", p.pkg, p.elapsed, limit))
		}
	}
	for _, pkg := range sortedKeys(r.counts) {
		for _, name := range sortedKeys(r.counts[pkg]) {
			if n, limit := r.counts[pkg][name], b.countLimit(pkg, name); n > limit {
				over = append(over, fmt.Sprintf("package %s counted %d %s, limit %d", pkg, n, name, limit))
			}
		}
	}
	return over
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type parallelism struct {
	pkg                string
	tests, wall, ratio float64
}

// parallelism returns, for each package that ran leaf tests, the sum of their times divided by the wall time of
// the package, lowest ratio first.
func (r *report) parallelism() []parallelism {
	sums := map[string]float64{}
	for _, t := range r.tests {
		if !r.parents[t.pkg+"\x00"+t.name] {
			sums[t.pkg] += t.elapsed
		}
	}
	var out []parallelism
	for _, p := range r.packages {
		if sum := sums[p.pkg]; sum > 0 && p.elapsed > 0 {
			out = append(out, parallelism{p.pkg, sum, p.elapsed, sum / p.elapsed})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ratio < out[j].ratio })
	return out
}

func appendFile(path, text string) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(text); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func summary(title string, r *report, over []string) string {
	if title == "" {
		title = "Test time"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### %s: slowest tests\n\n| Seconds | Package | Test |\n| ---: | --- | --- |\n", title)
	leaves := make([]result, 0, len(r.tests))
	for _, t := range r.tests {
		if !r.parents[t.pkg+"\x00"+t.name] {
			leaves = append(leaves, t)
		}
	}
	sort.Slice(leaves, func(i, j int) bool { return leaves[i].elapsed > leaves[j].elapsed })
	for _, t := range leaves[:min(10, len(leaves))] {
		fmt.Fprintf(&b, "| %.2f | `%s` | `%s` |\n", t.elapsed, t.pkg, t.name)
	}
	fmt.Fprint(&b, "\nSlowest packages\n\n| Seconds | Package |\n| ---: | --- |\n")
	pkgs := append([]result(nil), r.packages...)
	sort.Slice(pkgs, func(i, j int) bool { return pkgs[i].elapsed > pkgs[j].elapsed })
	for _, p := range pkgs[:min(5, len(pkgs))] {
		fmt.Fprintf(&b, "| %.2f | `%s` |\n", p.elapsed, p.pkg)
	}
	fmt.Fprint(&b, "\nParallelism: test seconds divided by package seconds\n\n| Ratio | Test seconds | Package seconds | Package |\n| ---: | ---: | ---: | --- |\n")
	for _, p := range r.parallelism() {
		fmt.Fprintf(&b, "| %.2f | %.2f | %.2f | `%s` |\n", p.ratio, p.tests, p.wall, p.pkg)
	}
	fmt.Fprint(&b, "\nCounts\n\n| Package | Name | Count |\n| --- | --- | ---: |\n")
	for _, pkg := range sortedKeys(r.counts) {
		for _, name := range sortedKeys(r.counts[pkg]) {
			if n := r.counts[pkg][name]; n > 0 {
				fmt.Fprintf(&b, "| `%s` | %s | %d |\n", pkg, name, n)
			}
		}
	}
	if r.note != "" {
		fmt.Fprintf(&b, "\n%s\n", r.note)
	}
	for _, line := range over {
		fmt.Fprintf(&b, "\nOver budget: %s\n", line)
	}
	b.WriteString("\n")
	return b.String()
}

// flushPackage drops the held output of every test of pkg, and prints it when
// the package failed: a timeout or a panic fails the package without a fail
// event for the test that printed the trace.
func flushPackage(out io.Writer, held map[string][]string, pkg string, failed bool) {
	keys := make([]string, 0, len(held))
	for key := range held {
		if strings.HasPrefix(key, pkg+"\x00") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		if failed {
			for _, text := range held[key] {
				_, _ = fmt.Fprint(out, text)
			}
		}
		delete(held, key)
	}
}
