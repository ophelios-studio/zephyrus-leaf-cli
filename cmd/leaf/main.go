// Leaf CLI entry point.
//
// This is a skeleton. The real implementation will embed FrankenPHP and the
// bundled leaf-core phar, dispatch to subcommands (init, dev, build, eject),
// and overlay the user's project directory on top of the embedded defaults.
//
// See README.md for the design.
package main

import (
	"fmt"
	"os"
)

const version = "0.0.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version", "-v", "--version":
		fmt.Println("leaf", version)
	case "help", "-h", "--help":
		usage()
	case "init", "dev", "build", "eject":
		fmt.Fprintf(os.Stderr, "leaf %s: not yet implemented\n", os.Args[1])
		os.Exit(1)
	default:
		fmt.Fprintf(os.Stderr, "leaf: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `leaf - a zero-dependency static site CLI

Usage:
    leaf <command> [flags]

Commands:
    init [name]    Scaffold a new site
    dev            Serve with live reload
    build          Generate static HTML into dist/
    eject          Convert to the full Composer project path
    version        Print version
    help           Show this help`)
}
