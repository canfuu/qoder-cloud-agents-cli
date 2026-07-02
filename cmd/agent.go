package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/canfuu/qoder-cloud-agents-cli/internal/api"
	"github.com/canfuu/qoder-cloud-agents-cli/internal/output"
	"github.com/spf13/cobra"
)

var agentCmd = &cobra.Command{
	Use:     "agent",
	Aliases: []string{"agents"},
	Short:   "Manage agents",
}

var agentListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")

		path := fmt.Sprintf("/agents?limit=%d", limit)
		data, err := client.Get(path)
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
				Model     string `json:"model"`
				Version   int    `json:"version"`
				CreatedAt string `json:"created_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			output.JSON(data)
			return nil
		}

		headers := []string{"ID", "NAME", "MODEL", "VERSION", "CREATED"}
		var rows [][]string
		for _, a := range resp.Data {
			rows = append(rows, []string{
				a.ID, a.Name, a.Model, fmt.Sprintf("%d", a.Version), a.CreatedAt[:10],
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var agentGetCmd = &cobra.Command{
	Use:   "get <agent-id>",
	Short: "Get agent details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		data, err := client.Get("/agents/" + args[0])
		if err != nil {
			return err
		}
		output.JSON(data)
		return nil
	},
}

var agentCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		name, _ := cmd.Flags().GetString("name")
		model, _ := cmd.Flags().GetString("model")
		system, _ := cmd.Flags().GetString("system")
		tools, _ := cmd.Flags().GetStringSlice("tools")

		body := map[string]interface{}{
			"name":  name,
			"model": model,
		}
		if system != "" {
			body["system"] = system
		}
		if len(tools) > 0 {
			body["tools"] = []map[string]interface{}{
				{
					"type":          "agent_toolset_20260401",
					"enabled_tools": tools,
				},
			}
		}

		data, err := client.Post("/agents", body)
		if err != nil {
			return err
		}

		var resp struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(data, &resp); err == nil {
			fmt.Printf("✓ Agent created: %s (%s)\n", resp.Name, resp.ID)
		}
		output.JSON(data)
		return nil
	},
}

var agentUpdateCmd = &cobra.Command{
	Use:   "update <agent-id>",
	Short: "Update an agent",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		// First get current version
		getData, err := client.Get("/agents/" + args[0])
		if err != nil {
			return err
		}
		var current struct {
			Version int    `json:"version"`
			Name    string `json:"name"`
			Model   string `json:"model"`
			System  string `json:"system"`
		}
		json.Unmarshal(getData, &current)

		body := map[string]interface{}{
			"version": current.Version,
		}

		name, _ := cmd.Flags().GetString("name")
		model, _ := cmd.Flags().GetString("model")
		system, _ := cmd.Flags().GetString("system")

		if name != "" {
			body["name"] = name
		} else {
			body["name"] = current.Name
		}
		if model != "" {
			body["model"] = model
		} else {
			body["model"] = current.Model
		}
		if system != "" {
			body["system"] = system
		} else if current.System != "" {
			body["system"] = current.System
		}

		data, err := client.Post("/agents/"+args[0], body)
		if err != nil {
			return err
		}

		fmt.Println("✓ Agent updated")
		output.JSON(data)
		return nil
	},
}

func init() {
	agentListCmd.Flags().IntP("limit", "l", 20, "Number of agents to list")
	agentListCmd.Flags().Bool("json", false, "Output as JSON")

	agentCreateCmd.Flags().StringP("name", "n", "", "Agent name (required)")
	agentCreateCmd.Flags().StringP("model", "m", "ultimate", "Model identifier")
	agentCreateCmd.Flags().StringP("system", "s", "", "System prompt")
	agentCreateCmd.Flags().StringSlice("tools", []string{"Bash", "Read", "Write", "Edit", "Glob", "Grep"}, "Enabled tools")
	agentCreateCmd.MarkFlagRequired("name")

	agentUpdateCmd.Flags().StringP("name", "n", "", "New agent name")
	agentUpdateCmd.Flags().StringP("model", "m", "", "New model identifier")
	agentUpdateCmd.Flags().StringP("system", "s", "", "New system prompt")

	agentCmd.AddCommand(agentListCmd)
	agentCmd.AddCommand(agentGetCmd)
	agentCmd.AddCommand(agentCreateCmd)
	agentCmd.AddCommand(agentUpdateCmd)
}
