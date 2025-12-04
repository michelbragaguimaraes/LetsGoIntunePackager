package main

import "github.com/michelbragaguimaraes/LetsGoIntunePackager/cmd"

// Version information (set via ldflags during build)
var (
	version   = "dev"
	buildTime = "unknown"
)

func main() {
	cmd.SetVersionInfo(version, buildTime)
	cmd.Execute()
}
