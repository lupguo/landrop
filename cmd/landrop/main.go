package main

import (
	"fmt"
	"os"

	"github.com/lupguo/landrop/internal/version"
)

func main() {
	for _, a := range os.Args[1:] {
		if a == "--version" || a == "-version" {
			fmt.Println(version.String())
			return
		}
	}
	fmt.Fprintln(os.Stderr, "usage: landrop [--version]")
	os.Exit(2)
}
