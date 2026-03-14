package main

import (
	"log"
	"os"

	"github.com/yegor-usoltsev/cronctl/internal/cli"
)

func main() {
	os.Exit(run()) //nolint:forbidigo // main entry point requires os.Exit
}

func run() int {
	log.SetFlags(0)
	return cli.Run(os.Args[1:])
}
