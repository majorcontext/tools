package seams

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/majorcontext/tools/testcount"
)

func readAllow(t testing.TB, root, name string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(root, "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseAllowlist(name, raw)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// gapsAtMergeBase reads the gap file at the merge base with GATES_BASE_REF (default origin/main). A missing base
// skips the shrink check, unless SEAMS_REQUIRED is 1.
func gapsAtMergeBase(t testing.TB, root string) (map[string]string, bool) {
	t.Helper()
	ref := os.Getenv("GATES_BASE_REF")
	if ref == "" {
		ref = "origin/main"
	}
	sha, err := testcount.Command(t, "git", "-C", root, "merge-base", "HEAD", ref).Output()
	if err != nil {
		if os.Getenv("SEAMS_REQUIRED") == "1" {
			t.Fatalf("find the merge base of HEAD and %s: %v", ref, err)
		}
		return nil, false
	}
	raw, err := testcount.Command(t, "git", "-C", root, "show", strings.TrimSpace(string(sha))+":testdata/seam-gaps.txt").Output()
	if err != nil {
		return nil, false
	}
	m, err := ParseAllowlist("seam-gaps.txt", raw)
	if err != nil {
		t.Fatal(err)
	}
	return m, true
}

// CheckRepository checks the module at root against the seams table of root/docs/architecture.md and the lists
// testdata/seam-gaps.txt, testdata/provider-sdk-importers.txt and testdata/provider-hosts.txt, and reports every
// violation as an error of t. A test of the module calls it with the path of the module root.
func CheckRepository(t testing.TB, root string) {
	t.Helper()
	m, err := Load(os.DirFS(root))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := os.ReadFile(filepath.Join(root, "docs", "architecture.md"))
	if err != nil {
		t.Fatal(err)
	}
	seams, err := ParseTable(doc)
	if err != nil {
		t.Fatal(err)
	}
	atBase, haveBase := gapsAtMergeBase(t, root)
	hits, err := HostHits(m)
	if err != nil {
		t.Fatal(err)
	}
	var all []string
	all = append(all, CheckTable(m, seams)...)
	all = append(all, CheckGaps(seams, readAllow(t, root, "seam-gaps.txt"), atBase, haveBase)...)
	all = append(all, CheckSDKImports(m, ImplDirs(seams), readAllow(t, root, "provider-sdk-importers.txt"))...)
	all = append(all, CheckHosts(hits, readAllow(t, root, "provider-hosts.txt"))...)
	for _, v := range all {
		t.Error(v)
	}
}
