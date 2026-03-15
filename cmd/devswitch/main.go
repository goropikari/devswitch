package main

import (
	"os"

	"github.com/goropikari/devswitch/internal/devswitch"
)

func main() {
	if err := devswitch.Execute(); err != nil {
		os.Exit(1)
	}
}
