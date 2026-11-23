package main

import "onecx.tkit.org/ocp/cmd"

var (
	// Used for flags.
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	cmd.Main(cmd.BuildVersion{
		Version: version,
		Commit:  commit,
		Date:    date,
	})
}
