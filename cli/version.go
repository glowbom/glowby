package main

import "fmt"

func runVersion() int {
	fmt.Printf("glowby %s (commit: %s, built: %s)\n", version, commit, date)
	return 0
}
