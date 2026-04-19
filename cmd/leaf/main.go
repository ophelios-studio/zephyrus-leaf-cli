// Leaf CLI entry point.
//
// Dispatches subcommands. Actual logic lives under internal/. Keep this file
// thin; it should only route flags to the right package.
package main

import (
	"fmt"
	"os"
)

const version = "0.0.0-dev"

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "version", "-v", "--version":
		fmt.Println("leaf", version)
	case "help", "-h", "--help":
		usage(os.Stdout)
	case "build":
		os.Exit(runBuild(os.Args[2:]))
	case "dev":
		os.Exit(runDev(os.Args[2:]))
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "eject":
		os.Exit(runEject(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "leaf: unknown command %q\n", os.Args[1])
		usage(os.Stderr)
		os.Exit(2)
	}
}

func usage(w *os.File) {
	fmt.Fprintln(w, `leaf - a zero-dependency static site CLI

Usage:
    leaf <command> [flags]

Commands:
    init <name>    Scaffold a new site
    dev            Serve with live reload
    build          Generate static HTML into dist/
    eject          Convert to the full Composer project path
    version        Print version
    help           Show this help`)
}
