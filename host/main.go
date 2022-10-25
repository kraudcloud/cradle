// Copyright (c) 2020-present devguard GmbH
//
// please don't use this in production
// it's a quick hack to simulate vmm, NOT the vmm

package main

import (
	"github.com/spf13/cobra"
)

func main() {

	rootCmd := &cobra.Command{
		Use:   "kcradle",
		Short: "cradle vmm host simulator",
	}

	cc := &cobra.Command{
		Use:   "summon [cacheDir] [dockerImage]",
		Short: "prepare a local checkout for launch",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			summon(args[0], args[1])
		},
	}
	rootCmd.AddCommand(cc)

	cc = &cobra.Command{
		Use:   "run [cacheDir]",
		Short: "run the vmm host simulator",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			run(args[0])
		},
	}

	rootCmd.AddCommand(cc)

	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}

}
