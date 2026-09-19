// Package admin provides explicit local administration for standalone Starmap servers.
package admin

import (
	"encoding/json"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/server/administration"
)

type application interface {
	AdministrationConfig() (administration.Config, error)
}

// NewCommand creates local administration commands that share the server's private state.
// Stop the server before changing state through these commands.
func NewCommand(app application) *cobra.Command {
	command := &cobra.Command{Use: "admin", GroupID: "setup", Short: "Manage standalone server identities locally", Args: cobra.NoArgs}
	command.RunE = func(cmd *cobra.Command, _ []string) error { return cmd.Help() }
	for _, action := range []string{"init", "create", "rotate", "revoke", "recover", "status"} {
		child := &cobra.Command{Use: action, Short: "Run local administrator operation: " + action, Args: cobra.NoArgs}
		child.Flags().String("id", "operator", "identity name")
		if action == "create" {
			child.Flags().String("role", "subscriber", "identity role: subscriber or administrator")
		}
		if action == "rotate" {
			child.Flags().Duration("overlap", 10*time.Minute, "old credential overlap (maximum 24 hours)")
		}
		child.RunE = func(cmd *cobra.Command, _ []string) error { return run(cmd, app, action) }
		command.AddCommand(child)
	}
	return command
}

func run(cmd *cobra.Command, app application, action string) error {
	cfg, err := app.AdministrationConfig()
	if err != nil {
		return err
	}
	id, _ := cmd.Flags().GetString("id")
	if action == "recover" {
		token, err := administration.RecoverAdministrator(cmd.Context(), cfg, id)
		if err != nil {
			return err
		}
		return writeCredential(cmd, id, cfg.Audience, token)
	}
	if action == "init" {
		manager, token, err := administration.Initialize(cmd.Context(), cfg, id)
		if err != nil {
			return err
		}
		defer func() { _ = manager.Close() }()
		return writeCredential(cmd, id, cfg.Audience, token)
	}
	manager, err := administration.Open(cmd.Context(), cfg)
	if err != nil {
		return err
	}
	defer func() { _ = manager.Close() }()
	if action == "status" {
		return json.NewEncoder(cmd.OutOrStdout()).Encode(manager.Status())
	}
	actor, accepted := manager.Authenticate(os.Getenv("STARMAP_ADMIN_TOKEN"), cfg.Audience)
	if !accepted || actor.Role() != administration.Administrator {
		return &errors.AuthenticationError{Method: "administrator", Message: "set STARMAP_ADMIN_TOKEN to an active administrator credential"}
	}
	var token string
	switch action {
	case "create":
		role, _ := cmd.Flags().GetString("role")
		token, err = manager.Create(cmd.Context(), actor, id, administration.Role(role))
	case "rotate":
		overlap, _ := cmd.Flags().GetDuration("overlap")
		token, err = manager.Rotate(cmd.Context(), actor, id, overlap)
	case "revoke":
		err = manager.Revoke(cmd.Context(), actor, id)
	}
	if err != nil {
		return err
	}
	if token != "" {
		return writeCredential(cmd, id, cfg.Audience, token)
	}
	return json.NewEncoder(cmd.OutOrStdout()).Encode(manager.Status())
}

func writeCredential(cmd *cobra.Command, id, audience, token string) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
		ID         string `json:"id"`
		Audience   string `json:"audience"`
		Credential string `json:"credential"`
	}{id, audience, token})
}
