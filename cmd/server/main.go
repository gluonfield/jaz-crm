package main

import (
	"fmt"
	"os"
)

func main() {
	args := os.Args[1:]
	command := "serve"
	if len(args) > 0 && args[0][0] != '-' {
		command, args = args[0], args[1:]
	}
	var err error
	switch command {
	case "serve":
		err = runServe(args)
	case "apikey":
		err = runAPIKey(args)
	case "help", "-h", "--help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", command, err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: server [command] [flags]

commands:
  serve           run the API and web server (default)
  apikey EMAIL [WORKSPACE]
                  mint an API key for an existing user, in WORKSPACE (name or ID)
                  when they belong to several
`)
}
