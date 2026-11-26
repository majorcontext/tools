// Command lint runs the repository analyzers. Use: go tool lint ./...
package main

import (
	"golang.org/x/tools/go/analysis/multichecker"

	"github.com/majorcontext/tools/lint/paralleltests"
	"github.com/majorcontext/tools/lint/wallclock"
)

func main() { multichecker.Main(wallclock.Analyzer, paralleltests.Analyzer) }
