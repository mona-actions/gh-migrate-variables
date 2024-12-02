package main

import (
	"os"

	"github.com/mona-actions/gh-migrate-variables/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
