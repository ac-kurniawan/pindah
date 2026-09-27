package main

import (
	"os"

	"github.com/arcaku-labs/pindah/internal/cli"
)

func main() {
	os.Exit(int(cli.Run(cli.Env{Args: os.Args[1:]})))
}
