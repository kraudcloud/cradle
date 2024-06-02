// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"fmt"
	"github.com/kraudcloud/cradle/spec"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"
)

type VM struct {
	ID      string
	WorkDir string

	Launch spec.Launch

	layerCount int

	CradleGuest spec.Cradle

	// qemu
	Cmd *exec.Cmd

	// virtiofsd
	Filesystems []*exec.Cmd

	// sdn
	EthHostSideMac  [6]byte
	EthHostSideName string
	Ip4             *net.IPNet
	Ip6             *net.IPNet
	Gw4             net.IP
	Gw6             net.IP

	// reporter keepalive
	Stage atomic.Uint32

	PodNetwork *PodNetwork
}

var log = logrus.WithField("prefix", "vmm")

func RunCMD() *cobra.Command {

	var arg_cradle string
	var arg_instance uint16
	var arg_spec string

	var arg_mem int
	var arg_cpu int

	runCmd := &cobra.Command{
		Use:   "run [command]",
		Short: "run",
		Run: func(cmd *cobra.Command, args []string) {

			defer log.Info("teardown complete")

			cro := spec.CradleLaunchIntent{}
			err := yaml.Unmarshal([]byte(arg_spec), &cro)
			if err != nil {
				panic(err)
			}

			vm := &VM{
				ID: cro.Spec.ID,
				Launch: spec.Launch{
					ID:         cro.Spec.ID,
					Containers: cro.Spec.Containers,
					Resources:  cro.Spec.Resources,
				},
				WorkDir: fmt.Sprintf("/var/run/cradle/pods/%s/%d", cro.Spec.ID, arg_instance),
			}

			err = vm.SetupWorkDir()
			if err != nil {
				//TODO panics are not reported as vm log yet
				panic(err)
			}
			defer vm.Cleanup()

			vm.Launch.Resources.Cpu = arg_cpu
			vm.Launch.Resources.Mem = arg_mem

			log.Println("downloading images")
			for i, ctr := range vm.Launch.Containers {
				ctr, err := vm.DownloadImage(cmd.Context(), ctr.Image.Ref, "")
				if err != nil {
					panic(err)
				}
				vm.Launch.Containers[i] = *ctr
			}

			log.Println("setting up pod network")
			err = vm.StartNetwork()
			if err != nil {
				panic(err)
			}
			defer vm.StopNetwork()

			log.Println("prepare cradle")
			err = vm.PrepareCradleGuest(arg_cradle)
			if err != nil {
				panic(err)
			}

			log.Println("make launch guest config")
			err = vm.MakeGuestLaunchConfig()
			if err != nil {
				panic(err)
			}

			log.Println("start filesystem")
			err = vm.StartFilesystems()
			if err != nil {
				panic(err)
			}
			defer vm.KillFilesystems()

			log.Println("start qemu")
			err = vm.LaunchQemu()
			if err != nil {
				panic(err)
			}
			defer vm.KillQemu()

			err = vm.SetupNetworkPostLaunch()
			if err != nil {
				panic(err)
			}

			log.Println("start vdocker")
			err = vm.StartVDocker()
			if err != nil {
				log.Errorln(err)
				panic(err)
			}

			go func() {
				sigc := make(chan os.Signal, 1)
				signal.Notify(sigc,
					syscall.SIGINT,
					syscall.SIGTERM,
					syscall.SIGQUIT)

				sig := <-sigc
				fmt.Println("TERMINATING")
				go func() {
					<-sigc
					os.Exit(1)
				}()

				// FIXME send signal via vdocker
				_ = sig

				time.Sleep(30 * time.Second)
				vm.Cmd.Process.Kill()
			}()

			log.Println("vm up")
			err = vm.Wait()
			if err == nil {
				log.Errorf("VM EXIT (code %d) ", vm.Cmd.ProcessState.ExitCode())
			} else {
				log.Errorf("VM EXIT (err %s) ", err)
			}
		},
	}
	runCmd.Flags().StringVar(&arg_spec, "inline", "", "launch intent cro as literal yaml")
	runCmd.MarkFlagRequired("inline")

	runCmd.Flags().StringVar(&arg_cradle, "cradle", "/cradle", "use cradle from pkg dir instead (for development)")

	runCmd.Flags().Uint16Var(&arg_instance, "instance", 0, "if multiple instances are running, this is a counter to distinguish them")

	return runCmd
}
