package modpath

import "testing"

func TestParse(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"module example.com/a\n\ngo 1.24\n":     "example.com/a",
		"// c\nmodule \"example.com/b\" // x\n": "example.com/b",
		"module\texample.com/c\n":               "example.com/c",
	} {
		if got, err := Parse([]byte(in)); err != nil || got != want {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := Parse([]byte("modules x\ngo 1.24\n")); err == nil {
		t.Error("Parse without a module line: no error")
	}
}
