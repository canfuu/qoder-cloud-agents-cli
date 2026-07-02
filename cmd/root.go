package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "qca",
	Short: "Qoder Cloud Agents CLI",
	Long:  "A CLI tool for managing Qoder Cloud Agents - agents, environments, and sessions.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.AddCommand(authCmd)
	rootCmd.AddCommand(agentCmd)
	rootCmd.AddCommand(envCmd)
	rootCmd.AddCommand(sessionCmd)
	rootCmd.AddCommand(modelCmd)

	rootCmd.Version = "0.1.0"
	rootCmd.SetVersionTemplate(fmt.Sprintf("qca version %s\n", "0.1.0"))
}
