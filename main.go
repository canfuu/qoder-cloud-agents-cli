package main

import (
	"os"

	"github.com/canfuu/qoder-cloud-agents-cli/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
