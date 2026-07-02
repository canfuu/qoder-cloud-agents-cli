package cmd

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/canfuu/qoder-cloud-agents-cli/internal/config"
	"github.com/spf13/cobra"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

var authLoginCmd = &cobra.Command{
	Use:   "login",
	Short: "Log in to Qoder Cloud Agents",
	Long:  "Authenticate with a Personal Access Token (PAT). The token is stored in ~/.config/qca/config.json",
	RunE: func(cmd *cobra.Command, args []string) error {
		token, _ := cmd.Flags().GetString("token")

		if token == "" {
			fmt.Print("Enter your Qoder Personal Access Token: ")
			reader := bufio.NewReader(os.Stdin)
			input, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("failed to read token: %w", err)
			}
			token = strings.TrimSpace(input)
		}

		if token == "" {
			return fmt.Errorf("token cannot be empty")
		}

		// Verify the token
		req, err := http.NewRequest("GET", config.DefaultAPIBase+"/agents?limit=1", nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return fmt.Errorf("failed to verify token: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == 401 {
			return fmt.Errorf("invalid token: authentication failed")
		}
		if resp.StatusCode >= 400 {
			return fmt.Errorf("token verification failed with status %d", resp.StatusCode)
		}

		cfg := &config.Config{
			Token:   token,
			APIBase: config.DefaultAPIBase,
		}
		if err := config.Save(cfg); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		fmt.Println("✓ Logged in successfully")
		fmt.Println("  Token stored in ~/.config/qca/config.json")
		return nil
	},
}

var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			fmt.Println("✗ Not logged in")
			return nil
		}

		// Mask token
		masked := cfg.Token[:8] + "..." + cfg.Token[len(cfg.Token)-4:]
		fmt.Println("✓ Logged in to Qoder Cloud Agents")
		fmt.Printf("  API: %s\n", cfg.APIBase)
		fmt.Printf("  Token: %s\n", masked)
		return nil
	},
}

var authLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Log out from Qoder Cloud Agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := &config.Config{}
		if err := config.Save(cfg); err != nil {
			return err
		}
		fmt.Println("✓ Logged out successfully")
		return nil
	},
}

func init() {
	authLoginCmd.Flags().StringP("token", "t", "", "Personal Access Token")
	authCmd.AddCommand(authLoginCmd)
	authCmd.AddCommand(authStatusCmd)
	authCmd.AddCommand(authLogoutCmd)
}
