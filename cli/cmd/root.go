package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "warpdrop",
	Short: "Peer-to-peer file transfer tool using WebRTC, with webapp support and cross-functional design",
	Long: `WarpDrop is a command-line tool for transferring files directly between devices using WebRTC technology. It eliminates the need for intermediaries, ensuring fast and secure file sharing. WarpDrop also includes a webapp interface for browser-based transfers and is designed to be cross-functional across different platforms and environments.`,
	Version: "v0.0.1",
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func ()  {
		for s := range sig {
			fmt.Println(s.String())
			os.Exit(0)
		}
	}()
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}


