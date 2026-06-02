package main

import (
	"fmt"
	"os"
	"runtime"
)

func main() {
	hostname, _ := os.Hostname()
	fmt.Printf("test-function works!\n")
	fmt.Printf("  host: %s\n", hostname)
	fmt.Printf("  go:   %s\n", runtime.Version())
	fmt.Printf("  os:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
