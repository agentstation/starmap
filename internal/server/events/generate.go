//go:build ignore

package main

import (
	"log"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("gomarkdoc", "--output", "README.md", ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("gomarkdoc failed: %v", err)
	}

	log.Println("Generated README.md for events package")
}
