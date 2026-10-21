//go:build !desktop

// Command desktop is Mesa as a desktop app. It needs native libraries, so it
// is built only with the desktop tag; see main.go.
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "mesa-desktop is built with: go build -tags desktop,production,webkit2_41 -o bin/mesa-desktop ./cmd/desktop")
	fmt.Fprintln(os.Stderr, "(or `make desktop`; Linux needs the WebKitGTK 4.1 and GTK 3 development packages)")
	os.Exit(2)
}
