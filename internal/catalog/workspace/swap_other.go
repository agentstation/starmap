//go:build !darwin && !linux

package workspace

import "github.com/agentstation/starmap/pkg/errors"

func promoteExistingDirectory(_, target string) error {
	return &errors.ValidationError{Field: "workspace_replacement", Value: target, Message: "directory exchange is unsupported; use the replacement journal"}
}
