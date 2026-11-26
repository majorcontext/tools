// Package modpath reads the module path of the go.mod file in a directory.
package modpath

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Read returns the module path that dir/go.mod declares.
func Read(dir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return "", err
	}
	return Parse(data)
}

// Parse returns the module path that the go.mod content data declares.
func Parse(data []byte) (string, error) {
	s := bufio.NewScanner(bytes.NewReader(data))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		rest, ok := strings.CutPrefix(line, "module")
		if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
			continue
		}
		rest, _, _ = strings.Cut(rest, "//")
		rest = strings.TrimSpace(rest)
		if unq, err := strconv.Unquote(rest); err == nil {
			rest = unq
		}
		if rest != "" {
			return rest, nil
		}
	}
	return "", fmt.Errorf("go.mod declares no module")
}
