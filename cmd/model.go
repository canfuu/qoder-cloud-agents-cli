package cmd

import (
	"github.com/canfuu/qoder-cloud-agents-cli/internal/api"
	"github.com/canfuu/qoder-cloud-agents-cli/internal/output"
	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:     "model",
	Aliases: []string{"models"},
	Short:   "List available models",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		data, err := client.Get("/models")
		if err != nil {
			return err
		}
		output.JSON(data)
		return nil
	},
}
