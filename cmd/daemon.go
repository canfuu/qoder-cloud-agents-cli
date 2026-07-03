package cmd

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/canfuu/qoder-cloud-agents-cli/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Self-hosted worker daemon",
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the self-hosted worker daemon",
	Long: `Start a foreground worker that polls a self_hosted environment for work,
executes tools locally, and returns results to the Cloud Agents platform.

The worker runs until interrupted with Ctrl+C, at which point it gracefully
stops any active work items before exiting.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		envID, _ := cmd.Flags().GetString("environment-id")
		workdir, _ := cmd.Flags().GetString("workdir")
		name, _ := cmd.Flags().GetString("name")

		if workdir == "" {
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			workdir = wd
		}

		if envID == "" {
			return fmt.Errorf("--environment-id is required")
		}

		// Generate stable worker ID
		workerID := name
		if workerID == "" {
			hostname, _ := os.Hostname()
			h := sha256.Sum256([]byte(hostname + envID))
			workerID = fmt.Sprintf("qca-%s-%x", hostname, h[:4])
		}

		worker, err := daemon.NewWorker(envID, workerID, workdir)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		go func() {
			<-sigCh
			fmt.Printf("\n[signal] shutting down...\n")
			cancel()
		}()

		return worker.Run(ctx)
	},
}

func init() {
	daemonStartCmd.Flags().String("environment-id", "", "Self-hosted environment ID (required)")
	daemonStartCmd.Flags().String("workdir", "", "Working directory for tool execution (default: current directory)")
	daemonStartCmd.Flags().String("name", "", "Worker name (default: auto-generated from hostname)")

	daemonCmd.AddCommand(daemonStartCmd)
}
