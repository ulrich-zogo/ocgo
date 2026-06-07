package main

import (
	"os"

	"ocgo/internal/app"
)

var version = "dev"

func main() {
	if err := app.NewRootCommand(version).Execute(); err != nil {
		os.Exit(1)
	}
}
