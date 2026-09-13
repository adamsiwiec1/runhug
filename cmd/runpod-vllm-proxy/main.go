package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/cli"
)

func main() {
	if err := cli.Run(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, cli.FormatError(err.Error()))
		os.Exit(1)
	}
}
