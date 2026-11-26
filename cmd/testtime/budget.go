package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type testBudget struct {
	pkg    string
	prefix string
	limit  float64
}

type budget struct {
	leaf      float64
	pkgs      map[string]float64
	pkgBase   float64
	overrides []testBudget
	// counts maps PATH or "default", then NAME, to the largest count.
	counts map[string]map[string]int
	// calibrate is the calibration time of the reference runner. factor multiplies every time limit; 0 means 1.
	calibrate float64
	factor    float64
}

// readBudget parses the budget file. Each line is one of:
//
//	leaf SECONDS                 limit of a test with no subtests
//	package PATH|default SECONDS limit of the wall time of a package; PATH@JOB sets it for one job
//	test PATH NAME SECONDS       limit of the tests whose name starts with NAME
//	count PATH|default NAME MAX  limit of one count of a package; a package without a line and a default has the limit 0
//	calibrate SECONDS            calibration time of the reference runner
func readBudget(path string) (*budget, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	b := &budget{pkgs: map[string]float64{}, counts: map[string]map[string]int{}}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		bad := fmt.Errorf("%s: bad line %q", path, line)
		if len(f) < 2 {
			return nil, bad
		}
		if f[0] == "count" {
			max, perr := strconv.Atoi(f[len(f)-1])
			if perr != nil || max < 0 || len(f) != 4 {
				return nil, bad
			}
			if b.counts[f[1]] == nil {
				b.counts[f[1]] = map[string]int{}
			}
			b.counts[f[1]][f[2]] = max
			continue
		}
		secs, perr := strconv.ParseFloat(f[len(f)-1], 64)
		if perr != nil || secs <= 0 {
			return nil, bad
		}
		switch {
		case f[0] == "leaf" && len(f) == 2:
			b.leaf = secs
		case f[0] == "calibrate" && len(f) == 2:
			b.calibrate = secs
		case f[0] == "package" && len(f) == 3 && f[1] == "default":
			b.pkgBase = secs
		case f[0] == "package" && len(f) == 3:
			b.pkgs[f[1]] = secs
		case f[0] == "test" && len(f) == 4:
			b.overrides = append(b.overrides, testBudget{pkg: f[1], prefix: f[2], limit: secs})
		default:
			return nil, bad
		}
	}
	if b.leaf == 0 || b.pkgBase == 0 || b.calibrate == 0 {
		return nil, fmt.Errorf("%s: leaf, package default and calibrate are required", path)
	}
	return b, nil
}

func (b *budget) testLimit(pkg, name string) float64 {
	for _, o := range b.overrides {
		if o.pkg == pkg && strings.HasPrefix(name, o.prefix) {
			return o.limit * b.scale()
		}
	}
	return b.leaf * b.scale()
}

func (b *budget) scale() float64 {
	if b.factor == 0 {
		return 1
	}
	return b.factor
}

// factorFor is the factor of a machine whose calibration took measured seconds, times allowance for a sandbox that
// the calibration does not measure. A faster machine does not tighten the limits.
func factorFor(measured, reference, allowance float64) float64 {
	f := 1.0
	if reference > 0 {
		f = max(1, measured/reference)
	}
	return f * max(1, allowance)
}

// countLimit is the largest count of name in pkg.
func (b *budget) countLimit(pkg, name string) int {
	if v, ok := b.counts[pkg][name]; ok {
		return v
	}
	return b.counts["default"][name]
}

func (b *budget) packageLimit(pkg, job string) float64 {
	if v, ok := b.pkgs[pkg+"@"+job]; ok {
		return v * b.scale()
	}
	if v, ok := b.pkgs[pkg]; ok {
		return v * b.scale()
	}
	return b.pkgBase * b.scale()
}
