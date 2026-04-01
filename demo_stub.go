//go:build !demo

package main

import "fmt"

func runDemo() {
	fmt.Println("Demo mode not available.")
	fmt.Println("Rebuild with: go build -tags demo -o waxon-demo .")
}
