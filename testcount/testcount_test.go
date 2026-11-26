package testcount

import "testing"

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
