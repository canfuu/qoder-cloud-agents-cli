package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/canfuu/qoder-cloud-agents-cli/internal/api"
	"github.com/canfuu/qoder-cloud-agents-cli/internal/output"
	"github.com/spf13/cobra"
)

var envCmd = &cobra.Command{
	Use:     "env",
	Aliases: []string{"environment", "environments"},
	Short:   "Manage environments",
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all environments",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		jsonFlag, _ := cmd.Flags().GetBool("json")
		data, err := client.Get("/environments")
		if err != nil {
			return err
		}

		if jsonFlag {
			output.JSON(data)
			return nil
		}

		var resp struct {
			Data []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				Config    struct {
					Type       string `json:"type"`
					Networking struct {
						Type string `json:"type"`
					} `json:"networking"`
				} `json:"config"`
				CreatedAt string `json:"created_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			output.JSON(data)
			return nil
		}

		headers := []string{"ID", "NAME", "TYPE", "NETWORKING", "CREATED"}
		var rows [][]string
		for _, e := range resp.Data {
			rows = append(rows, []string{
				e.ID, e.Name, e.Config.Type, e.Config.Networking.Type, e.CreatedAt[:10],
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var envGetCmd = &cobra.Command{
	Use:   "get <env-id>",
	Short: "Get environment details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}
		data, err := client.Get("/environments/" + args[0])
		if err != nil {
			return err
		}
		output.JSON(data)
		return nil
	},
}

var envCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new environment",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		networking, _ := cmd.Flags().GetString("networking")
		envType, _ := cmd.Flags().GetString("type")

		config := map[string]interface{}{
			"type": envType,
		}
		if envType == "cloud" {
			config["networking"] = map[string]interface{}{
				"type": networking,
			}
		}

		body := map[string]interface{}{
			"name":   name,
			"config": config,
		}

		data, err := client.Post("/environments", body)
		if err != nil {
			return err
		}

		var resp struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &resp); err == nil {
			fmt.Printf("✓ Environment created: %s (%s)\n", resp.Name, resp.ID)
		}
		output.JSON(data)
		return nil
	},
}

var envUpdateCmd = &cobra.Command{
	Use:   "update <env-id>",
	Short: "Update an environment",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		networking, _ := cmd.Flags().GetString("networking")

		body := map[string]interface{}{}
		if name != "" {
			body["name"] = name
		}
		if networking != "" {
			body["config"] = map[string]interface{}{
				"type": "cloud",
				"networking": map[string]interface{}{
					"type": networking,
				},
			}
		}

		data, err := client.Post("/environments/"+args[0], body)
		if err != nil {
			return err
		}
		fmt.Println("✓ Environment updated")
		output.JSON(data)
		return nil
	},
}

func init() {
	envListCmd.Flags().Bool("json", false, "Output as JSON")

	envCreateCmd.Flags().StringP("name", "n", "", "Environment name (required)")
	envCreateCmd.Flags().String("networking", "unrestricted", "Networking policy: unrestricted, limited, allowed_hosts")
	envCreateCmd.Flags().String("type", "cloud", "Environment type: cloud or self_hosted")
	envCreateCmd.MarkFlagRequired("name")

	envUpdateCmd.Flags().StringP("name", "n", "", "New environment name")
	envUpdateCmd.Flags().String("networking", "", "New networking policy")

	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envGetCmd)
	envCmd.AddCommand(envCreateCmd)
	envCmd.AddCommand(envUpdateCmd)
}
