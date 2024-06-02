package main

import (
	"github.com/kraudcloud/cradle/vmm"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "cradle",
	}

	rootCmd.AddCommand(vmm.RunCMD())

	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}
}
