package cmd

import (
	"fmt"
	"os"
	"os/exec"
)

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func runCommandQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

func Elevate() {
	if os.Geteuid() != 0 {
		fmt.Println("This command must be run as root (or with sudo).")
		fmt.Println("Attempting to elevate permissions with sudo...")

		executable, err := os.Executable()
		if err != nil {
			fmt.Printf("Failed to get executable path: %v\n", err)
			os.Exit(1)
		}

		args := append([]string{executable}, os.Args[1:]...)
		childCmd := exec.Command("sudo", args...)
		childCmd.Stdin = os.Stdin
		childCmd.Stdout = os.Stdout
		childCmd.Stderr = os.Stderr

		err = childCmd.Run()
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			os.Exit(1)
		}
		os.Exit(0)
	}
}
