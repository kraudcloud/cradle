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

	var vfsVolumes []string
	var blockVolumes []string
	cc := &cobra.Command{
		Use:   "summon [cacheDir] [dockerImage]",
		Short: "prepare a local checkout for launch",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			summon(args[0], args[1], vfsVolumes, blockVolumes)
		},
	}

	cc.Flags().StringArrayVar(&vfsVolumes, "fs", []string{}, "add a file system volume to this container path")
	cc.Flags().StringArrayVar(&blockVolumes, "block", []string{}, "add a block volume to this container path")

	rootCmd.AddCommand(cc)

	cc = &cobra.Command{
		Use:   "run [runDir]",
		Short: "run the vmm host simulator",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			vm, err := New(args[0])
			if err != nil {
				panic(err)
			}
			vm.Run()
		},
	}

	rootCmd.AddCommand(cc)

	err := rootCmd.Execute()
	if err != nil {
		panic(err)
	}

}
