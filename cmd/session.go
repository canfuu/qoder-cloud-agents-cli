package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/canfuu/qoder-cloud-agents-cli/internal/api"
	"github.com/canfuu/qoder-cloud-agents-cli/internal/output"
	"github.com/spf13/cobra"
)

var sessionCmd = &cobra.Command{
	Use:     "session",
	Aliases: []string{"sessions"},
	Short:   "Manage sessions",
}

var sessionListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		jsonFlag, _ := cmd.Flags().GetBool("json")

		path := fmt.Sprintf("/sessions?limit=%d", limit)
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
				ID     string `json:"id"`
				Status string `json:"status"`
				Title  string `json:"title"`
				Agent  struct {
					Name string `json:"name"`
				} `json:"agent"`
				CreatedAt string `json:"created_at"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &resp); err != nil {
			output.JSON(data)
			return nil
		}

		headers := []string{"ID", "STATUS", "AGENT", "TITLE", "CREATED"}
		var rows [][]string
		for _, s := range resp.Data {
			title := s.Title
			if title == "" {
				title = "-"
			}
			rows = append(rows, []string{
				s.ID, s.Status, s.Agent.Name, title, s.CreatedAt[:10],
			})
		}
		output.Table(headers, rows)
		return nil
	},
}

var sessionGetCmd = &cobra.Command{
	Use:   "get <session-id>",
	Short: "Get session details",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}
		data, err := client.Get("/sessions/" + args[0])
		if err != nil {
			return err
		}
		output.JSON(data)
		return nil
	},
}

var sessionCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new session",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		agentID, _ := cmd.Flags().GetString("agent")
		envID, _ := cmd.Flags().GetString("env")
		title, _ := cmd.Flags().GetString("title")

		body := map[string]interface{}{
			"agent":          agentID,
			"environment_id": envID,
		}
		if title != "" {
			body["title"] = title
		}

		data, err := client.Post("/sessions", body)
		if err != nil {
			return err
		}

		var resp struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		}
		if err := json.Unmarshal(data, &resp); err == nil {
			fmt.Printf("✓ Session created: %s (status: %s)\n", resp.ID, resp.Status)
		}
		output.JSON(data)
		return nil
	},
}

var sessionSendCmd = &cobra.Command{
	Use:   "send <session-id> <message>",
	Short: "Send a message to a session",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		sessionID := args[0]
		var message string
		if len(args) > 1 {
			message = strings.Join(args[1:], " ")
		} else {
			return fmt.Errorf("message is required")
		}

		body := map[string]interface{}{
			"events": []map[string]interface{}{
				{
					"type": "user.message",
					"content": []map[string]interface{}{
						{"type": "text", "text": message},
					},
				},
			},
		}

		data, err := client.Post("/sessions/"+sessionID+"/events", body)
		if err != nil {
			return err
		}

		fmt.Println("✓ Message sent")
		output.JSON(data)
		return nil
	},
}

var sessionStreamCmd = &cobra.Command{
	Use:   "stream <session-id>",
	Short: "Stream events from a session (SSE)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		body, err := client.StreamGet("/sessions/" + args[0] + "/events/stream")
		if err != nil {
			return err
		}
		defer body.Close()

		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			// Parse SSE format
			if strings.HasPrefix(line, "data: ") {
				data := strings.TrimPrefix(line, "data: ")
				if data == "{}" {
					continue // heartbeat
				}
				var event map[string]interface{}
				if err := json.Unmarshal([]byte(data), &event); err == nil {
					eventType, _ := event["type"].(string)
					switch eventType {
					case "agent.message":
						content, _ := event["content"].([]interface{})
						for _, c := range content {
							block, _ := c.(map[string]interface{})
							if text, ok := block["text"].(string); ok {
								fmt.Println(text)
							}
						}
					case "agent.thinking":
						fmt.Println("[thinking...]")
					case "agent.tool_use":
						name, _ := event["name"].(string)
						fmt.Printf("[tool: %s]\n", name)
					case "agent.tool_result":
						isError, _ := event["is_error"].(bool)
						if isError {
							fmt.Println("[tool error]")
						}
					case "session.status_idle":
						fmt.Println("\n--- Session idle ---")
						return nil
					case "session.status_running":
						fmt.Println("--- Session running ---")
					default:
						if eventType != "" {
							fmt.Printf("[%s]\n", eventType)
						}
					}
				}
			} else if strings.HasPrefix(line, "event: ") {
				// event type line, skip (we parse from data)
			} else if strings.HasPrefix(line, "id: ") {
				// event id, skip
			}
		}
		return scanner.Err()
	},
}

var sessionEventsCmd = &cobra.Command{
	Use:   "events <session-id>",
	Short: "List events for a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		limit, _ := cmd.Flags().GetInt("limit")
		path := fmt.Sprintf("/sessions/%s/events?limit=%d", args[0], limit)
		data, err := client.Get(path)
		if err != nil {
			return err
		}
		output.JSON(data)
		return nil
	},
}

var sessionCancelCmd = &cobra.Command{
	Use:   "cancel <session-id>",
	Short: "Cancel the current turn of a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		data, err := client.Post("/sessions/"+args[0]+"/cancel", nil)
		if err != nil {
			return err
		}
		fmt.Println("✓ Session cancelled")
		output.JSON(data)
		return nil
	},
}

var sessionArchiveCmd = &cobra.Command{
	Use:   "archive <session-id>",
	Short: "Archive a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		data, err := client.Post("/sessions/"+args[0]+"/archive", nil)
		if err != nil {
			return err
		}
		fmt.Println("✓ Session archived")
		output.JSON(data)
		return nil
	},
}

var sessionDeleteCmd = &cobra.Command{
	Use:   "delete <session-id>",
	Short: "Delete a session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}

		data, err := client.Delete("/sessions/" + args[0])
		if err != nil {
			return err
		}
		fmt.Println("✓ Session deleted")
		output.JSON(data)
		return nil
	},
}

var sessionChatCmd = &cobra.Command{
	Use:   "chat <session-id>",
	Short: "Interactive chat with a session",
	Long:  "Start an interactive chat session. Send messages and see responses in real-time.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := api.NewClient()
		if err != nil {
			return err
		}
		sessionID := args[0]

		fmt.Println("Interactive chat (type 'exit' or Ctrl+C to quit)")
		fmt.Println("---")

		reader := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("\n> ")
			input, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					return nil
				}
				return err
			}
			input = strings.TrimSpace(input)
			if input == "" {
				continue
			}
			if input == "exit" || input == "quit" {
				return nil
			}

			// Send message
			body := map[string]interface{}{
				"events": []map[string]interface{}{
					{
						"type": "user.message",
						"content": []map[string]interface{}{
							{"type": "text", "text": input},
						},
					},
				},
			}
			_, err = client.Post("/sessions/"+sessionID+"/events", body)
			if err != nil {
				fmt.Printf("Error sending message: %v\n", err)
				continue
			}

			// Stream response
			stream, err := client.StreamGet("/sessions/" + sessionID + "/events/stream")
			if err != nil {
				fmt.Printf("Error streaming: %v\n", err)
				continue
			}

			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data: ") {
					continue
				}
				data := strings.TrimPrefix(line, "data: ")
				if data == "{}" {
					continue
				}
				var event map[string]interface{}
				if err := json.Unmarshal([]byte(data), &event); err != nil {
					continue
				}
				eventType, _ := event["type"].(string)
				switch eventType {
				case "agent.message":
					content, _ := event["content"].([]interface{})
					for _, c := range content {
						block, _ := c.(map[string]interface{})
						if text, ok := block["text"].(string); ok {
							fmt.Println(text)
						}
					}
				case "session.status_idle":
					goto done
				}
			}
		done:
			stream.Close()
		}
	},
}

func init() {
	sessionListCmd.Flags().IntP("limit", "l", 20, "Number of sessions to list")
	sessionListCmd.Flags().Bool("json", false, "Output as JSON")

	sessionCreateCmd.Flags().StringP("agent", "a", "", "Agent ID (required)")
	sessionCreateCmd.Flags().StringP("env", "e", "", "Environment ID (required)")
	sessionCreateCmd.Flags().StringP("title", "t", "", "Session title")
	sessionCreateCmd.MarkFlagRequired("agent")
	sessionCreateCmd.MarkFlagRequired("env")

	sessionEventsCmd.Flags().IntP("limit", "l", 50, "Number of events to list")

	sessionCmd.AddCommand(sessionListCmd)
	sessionCmd.AddCommand(sessionGetCmd)
	sessionCmd.AddCommand(sessionCreateCmd)
	sessionCmd.AddCommand(sessionSendCmd)
	sessionCmd.AddCommand(sessionStreamCmd)
	sessionCmd.AddCommand(sessionEventsCmd)
	sessionCmd.AddCommand(sessionCancelCmd)
	sessionCmd.AddCommand(sessionArchiveCmd)
	sessionCmd.AddCommand(sessionDeleteCmd)
	sessionCmd.AddCommand(sessionChatCmd)
}
