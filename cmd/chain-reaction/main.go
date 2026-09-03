package main

import (
	"fmt"
	"os"

	"github.com/ashwnn/chain-reaction/internal/buildinfo"
	"github.com/ashwnn/chain-reaction/internal/cli"
)

func main() {
	cmd := cli.NewRootCmd(buildinfo.Version, buildinfo.Commit, buildinfo.Date)
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
