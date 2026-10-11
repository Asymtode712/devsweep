// devsweep finds and safely reclaims disk space used by developer tooling:
// node_modules, virtualenvs, build output, Docker, Xcode and local AI models.
package main

import (
	"os"

	"github.com/Asymtode712/devsweep/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:]))
}
