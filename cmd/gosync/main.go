package main

import (
	"fmt"
	"os"

	"github.com/mwantia/gosync/cmd/gosync/cli"
	"github.com/mwantia/gosync/cmd/gosync/cli/server"
)

var (
	version = "0.0.1-dev"
	commit  = "main"
)

func main() {
	root := cli.NewRootCommand(cli.VersionInfo{
		Version: version,
		Commit:  commit,
	})

	root.AddCommand(cli.NewVersionCommand())

	root.AddCommand(server.NewAgentCommand())
	root.AddCommand(server.NewConfigCommand())

	if err := root.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
