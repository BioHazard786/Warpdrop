package main

import (
	"github.com/BioHazard786/Warpdrop/cli/cmd"
	"github.com/BioHazard786/Warpdrop/cli/internal/logging"
)

func main() {
	// Initialize logging
	logging.Init()
	cmd.Execute()
}
