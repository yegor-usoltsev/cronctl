package main

import (
	"log"
	"os"

	"github.com/yegor-usoltsev/cronctl/internal/cli"
)

func main() {
	os.Exit(run()) //nolint:forbidigo
}

func run() (code int) {
	log.SetFlags(0)
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic: %v", r)
			code = 1
		}
	}()

	args := os.Args[1:]
	return cli.Run(args)
}
