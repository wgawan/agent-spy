package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: agent-spy [path]")
		os.Exit(1)
	}
	fmt.Printf("Watching: %s\n", os.Args[1])
}
