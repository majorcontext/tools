// Package paralleltests reports a top-level test that does not call t.Parallel and is not listed in
// testdata/serial-tests.txt.
package paralleltests

import (
	"sync"

	"bufio"
	"fmt"
	"github.com/majorcontext/tools/internal/modpath"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
)

// modulePath is the module of the working directory, where the analyzer runs, as it reads serialFile there.
var modulePath = sync.OnceValue(func() string { mod, _ := modpath.Read("."); return mod })

var serialFile = "testdata/serial-tests.txt"

// Analyzer reports a top-level TestXxx that does not call t.Parallel on its own t, directly or through a function of
// the same package that does. The file testdata/serial-tests.txt lists the exceptions as "package TestName reason"
// lines; a line without a reason, and a line for a test that does not exist or already runs in parallel, is reported.
var Analyzer = &analysis.Analyzer{
	Name: "paralleltests",
	Doc:  "report top-level tests that do not call t.Parallel and are not listed in the serial test file",
	Run:  run,
}

func init() {
	Analyzer.Flags.StringVar(&serialFile, "serial", serialFile, "file that lists the tests that stay serial")
}

func run(pass *analysis.Pass) (any, error) {
	var files []*ast.File
	for _, f := range pass.Files {
		if strings.HasSuffix(pass.Fset.File(f.Pos()).Name(), "_test.go") {
			files = append(files, f)
		}
	}
	if len(files) == 0 {
		return nil, nil
	}
	pkg := packageKey(pass.Pkg.Path())
	serial, err := readSerial(serialFile, pkg)
	if err != nil {
		return nil, err
	}
	helpers := parallelHelpers(pass, files)
	for _, f := range files {
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || !isTest(pass, fn) {
				continue
			}
			parallel := callsParallel(pass, fn.Body, paramObj(pass, fn.Type, 0), helpers)
			_, listed := serial[fn.Name.Name]
			switch {
			case parallel && listed:
				pass.Reportf(fn.Pos(), "%s calls t.Parallel but %s lists it: remove the line", fn.Name.Name, serialFile)
			case !parallel && !listed:
				pass.Reportf(fn.Pos(), "%s does not call t.Parallel: call it, or list the test in %s with a reason", fn.Name.Name, serialFile)
			}
		}
	}
	return nil, reportEntries(pass, files, serial)
}

// reportEntries reports the lines of the serial file that name a package without a reason or a test that the
// package does not declare. Only the pass that holds the first test file of the directory reports them.
func reportEntries(pass *analysis.Pass, files []*ast.File, serial map[string]string) error {
	dir := filepath.Dir(pass.Fset.File(files[0].Pos()).Name())
	names, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil || len(names) == 0 {
		return err
	}
	sort.Strings(names)
	var first *ast.File
	for _, f := range files {
		if pass.Fset.File(f.Pos()).Name() == names[0] {
			first = f
		}
	}
	if first == nil {
		return nil
	}
	declared := map[string]bool{}
	for _, name := range names {
		f, err := parser.ParseFile(token.NewFileSet(), name, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		for _, d := range f.Decls {
			if fn, ok := d.(*ast.FuncDecl); ok && fn.Recv == nil {
				declared[fn.Name.Name] = true
			}
		}
	}
	for _, name := range sortedNames(serial) {
		switch {
		case serial[name] == "":
			pass.Reportf(first.Package, "%s lists %s without a reason", serialFile, name)
		case !declared[name]:
			pass.Reportf(first.Package, "%s lists %s, which the package does not declare: remove the line", serialFile, name)
		}
	}
	return nil
}

func sortedNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func packageKey(path string) string {
	path = strings.TrimSuffix(path, "_test")
	mod := modulePath()
	if path == mod {
		return "."
	}
	return strings.TrimPrefix(path, mod+"/")
}

// readSerial returns the entries of the serial file for pkg, by test name.
func readSerial(path, pkg string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s: bad line %q: want \"package TestName reason\"", path, line)
		}
		if fields[0] == pkg {
			out[fields[1]] = strings.Join(fields[2:], " ")
		}
	}
	return out, sc.Err()
}

func isTest(pass *analysis.Pass, fn *ast.FuncDecl) bool {
	name := fn.Name.Name
	if fn.Recv != nil || fn.Body == nil || !strings.HasPrefix(name, "Test") || name == "TestMain" {
		return false
	}
	if rest := strings.TrimPrefix(name, "Test"); rest != "" && unicode.IsLower([]rune(rest)[0]) {
		return false
	}
	return paramObj(pass, fn.Type, 0) != nil && isTestingT(paramObj(pass, fn.Type, 0).Type())
}

func isTestingT(t types.Type) bool {
	p, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := p.Elem().(*types.Named)
	return ok && n.Obj().Pkg() != nil && n.Obj().Pkg().Path() == "testing" && n.Obj().Name() == "T"
}

// paramObj returns the variable of the i-th parameter of a function type, or nil.
func paramObj(pass *analysis.Pass, ft *ast.FuncType, i int) *types.Var {
	n := 0
	for _, field := range ft.Params.List {
		if len(field.Names) == 0 {
			n++
			continue
		}
		for _, id := range field.Names {
			if n == i {
				v, _ := pass.TypesInfo.Defs[id].(*types.Var)
				return v
			}
			n++
		}
	}
	return nil
}

// parallelHelpers returns the functions of the package that call t.Parallel on a *testing.T parameter, directly or
// through another such function, with the indexes of those parameters.
func parallelHelpers(pass *analysis.Pass, files []*ast.File) map[*types.Func]map[int]bool {
	out := map[*types.Func]map[int]bool{}
	for changed := true; changed; {
		changed = false
		for _, f := range files {
			for _, d := range f.Decls {
				fn, ok := d.(*ast.FuncDecl)
				if !ok || fn.Body == nil {
					continue
				}
				obj, _ := pass.TypesInfo.Defs[fn.Name].(*types.Func)
				if obj == nil {
					continue
				}
				for i := 0; i < fn.Type.Params.NumFields(); i++ {
					p := paramObj(pass, fn.Type, i)
					if p == nil || !isTestingT(p.Type()) || out[obj][i] || !callsParallel(pass, fn.Body, p, out) {
						continue
					}
					if out[obj] == nil {
						out[obj] = map[int]bool{}
					}
					out[obj][i] = true
					changed = true
				}
			}
		}
	}
	return out
}

// callsParallel reports whether body calls Parallel on the variable t, or passes t to a helper that does.
func callsParallel(pass *analysis.Pass, body ast.Node, t *types.Var, helpers map[*types.Func]map[int]bool) bool {
	if t == nil {
		return false
	}
	found := false
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || found {
			return !found
		}
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Parallel" && isVar(pass, sel.X, t) {
			found = true
			return false
		}
		if fn := callee(pass, call.Fun); fn != nil {
			for i, a := range call.Args {
				if helpers[fn][i] && isVar(pass, a, t) {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

func isVar(pass *analysis.Pass, e ast.Expr, v *types.Var) bool {
	id, ok := ast.Unparen(e).(*ast.Ident)
	return ok && pass.TypesInfo.Uses[id] == v
}

func callee(pass *analysis.Pass, fun ast.Expr) *types.Func {
	var id *ast.Ident
	switch e := ast.Unparen(fun).(type) {
	case *ast.Ident:
		id = e
	case *ast.SelectorExpr:
		id = e.Sel
	default:
		return nil
	}
	fn, _ := pass.TypesInfo.Uses[id].(*types.Func)
	return fn
}
