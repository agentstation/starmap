package auth

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"cloud.google.com/go/auth/credentials"
	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/cmd/starmap/application"
)

// NewGCloudCommand creates the auth gcloud subcommand using app context.
func NewGCloudCommand(app application.Application) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gcloud",
		Short: "Manage Google Cloud authentication",
		Long: `Authenticate with Google Cloud for Vertex AI access.

By default, checks if authenticated and runs 'gcloud auth application-default login' if needed.
Use --check to only verify status without authenticating.

Examples:
  starmap auth gcloud           # Authenticate if needed
  starmap auth gcloud --check   # Check status only (exit 0 if authenticated)
  starmap auth gcloud --force   # Force re-authentication
  starmap auth gcloud --project my-project  # Set default project`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGCloudAuth(cmd, args)
		},
	}

	cmd.Flags().Bool("check", false, "Only check status, don't authenticate")
	cmd.Flags().Bool("force", false, "Force re-authentication")
	cmd.Flags().String("project", "", "Set default project after auth")

	return cmd
}

func runGCloudAuth(cmd *cobra.Command, args []string) error {
	// This command doesn't take positional arguments
	if len(args) > 0 {
		return fmt.Errorf("unexpected argument: %s", args[0])
	}

	check, _ := cmd.Flags().GetBool("check")
	force, _ := cmd.Flags().GetBool("force")
	project, _ := cmd.Flags().GetString("project")

	ctx := context.Background()

	// Check current authentication status
	authenticated, projectID, _ := checkGCloudAuthentication(ctx)

	if check {
		// Only checking status
		if authenticated {
			fmt.Println("✅ Authenticated with Google Cloud")
			if projectID != "" {
				fmt.Printf("   Project: %s\n", projectID)
			}
			return nil
		}
		// Silent error for scripting (exit code 1)
		cmd.SilenceUsage = true
		return fmt.Errorf("not authenticated with Google Cloud")
	}

	// Show current status
	if authenticated && !force {
		fmt.Println("✅ Already authenticated with Google Cloud")
		if projectID != "" {
			fmt.Printf("   Current project: %s\n", projectID)
		}

		// If project specified, update it
		if project != "" && project != projectID {
			return setGCloudProject(project)
		}
		return nil
	}

	// Check if gcloud is installed
	if _, err := exec.LookPath("gcloud"); err != nil {
		return fmt.Errorf("gcloud CLI not found. Please install Google Cloud SDK: https://cloud.google.com/sdk/docs/install")
	}

	// Run authentication
	fmt.Println("Authenticating with Google Cloud...")
	fmt.Println("This will open your browser for authentication.")
	fmt.Println()

	authCmd := exec.CommandContext(ctx, "gcloud", "auth", "application-default", "login")
	authCmd.Stdout = os.Stdout
	authCmd.Stderr = os.Stderr
	authCmd.Stdin = os.Stdin

	if err := authCmd.Run(); err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	fmt.Println("\n✅ Successfully authenticated with Google Cloud")

	// Set project if specified
	if project != "" {
		return setGCloudProject(project)
	}

	// Check if we need to set a project
	_, currentProject, _ := checkGCloudAuthentication(ctx)
	if currentProject == "" {
		fmt.Println("\n⚠️  No default project set.")
		fmt.Println("Set one with: starmap auth gcloud --project YOUR_PROJECT_ID")
		fmt.Println("Or: gcloud config set project YOUR_PROJECT_ID")
	}

	return nil
}

func checkGCloudAuthentication(ctx context.Context) (bool, string, error) {
	// Try to get credentials using the auth package
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"},
	})
	if err != nil {
		return false, "", err
	}

	// Try to get a token to verify auth works
	token, err := creds.Token(ctx)
	if err != nil || token == nil {
		return false, "", err
	}

	// Try to get project ID
	var projectID string

	// First try quota project (set by gcloud auth application-default)
	if pid, err := creds.QuotaProjectID(ctx); err == nil && pid != "" {
		projectID = pid
	} else if pid, err := creds.ProjectID(ctx); err == nil && pid != "" {
		// Fall back to regular project ID
		projectID = pid
	}

	// Also check environment variable
	if projectID == "" {
		projectID = os.Getenv("GOOGLE_CLOUD_PROJECT")
	}

	return true, projectID, nil
}

func setGCloudProject(project string) error {
	fmt.Printf("Setting default project to: %s\n", project)

	ctx := context.Background()

	// Set using gcloud config
	cmd := exec.CommandContext(ctx, "gcloud", "config", "set", "project", project)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set project: %w\nOutput: %s", err, output)
	}

	// Also set quota project for ADC
	cmd = exec.CommandContext(ctx, "gcloud", "auth", "application-default", "set-quota-project", project)
	if output, err := cmd.CombinedOutput(); err != nil {
		// This is not fatal, just inform
		fmt.Printf("⚠️  Could not set quota project: %s\n", output)
	}

	fmt.Printf("✅ Default project set to: %s\n", project)
	return nil
}
