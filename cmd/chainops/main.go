package main

import (
	"os"

	"github.com/Pantani/gorchestrator/internal/cli"
)

func main() {
	os.Exit(cli.RunProgram("chainops", os.Args[1:]))
}
