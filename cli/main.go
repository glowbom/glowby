package main

import (
	"fmt"
	"os"
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

const usage = `glowby - terminal-first local AI coding agent

Usage:
  glowby code [project-path] [--show-local-auth]
                                Start Glowby from this checkout and open the browser
  glowby doctor                 Check environment dependencies
  glowby version                Print version info
  glowby help                   Show this help

Examples:
  glowby code                   Start Glowby from the current checkout
  glowby code --show-local-auth Start Glowby and print local dev auth credentials
  glowby code /path/to/project  Start Glowby and print a project path hint
`

func main() {
	args := os.Args[1:]
	os.Exit(run(args))
}

func run(args []string) int {
	if len(args) == 0 {
		fmt.Print(usage)
		return 0
	}

	switch args[0] {
	case "code":
		return runCode(args[1:])
	case "doctor":
		return runDoctor()
	case "version":
		return runVersion()
	case "help", "-h", "--help":
		fmt.Print(usage)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", args[0])
		fmt.Fprint(os.Stderr, usage)
		return 2
	}
}
