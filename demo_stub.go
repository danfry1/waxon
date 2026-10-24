//go:build !demo

package main

import "fmt"

// demoUsage is the help line for `waxon demo`: release builds have no demo
// mode, so they don't offer it.
const demoUsage = ""

func runDemo() {
	fmt.Println("Demo mode not available.")
	fmt.Println("Rebuild with: go build -tags demo -o waxon-demo .")
}
