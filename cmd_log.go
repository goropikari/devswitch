package main

import (
	"fmt"
	"os"
)

func warnErr(action string, err error) {
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "WARN: %s: %v\n", action, err)
	}
}
