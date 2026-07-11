package update

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	pkgerrors "github.com/agentstation/starmap/pkg/errors"
	"github.com/agentstation/starmap/pkg/sources"
	"github.com/agentstation/starmap/pkg/sync"
)

func promptForMissingDependency(ctx context.Context, sourceID sources.ID, dependency sources.Dependency, _ bool) (sync.DependencyDecision, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	fmt.Printf("\n⚠️  Missing Dependency: %s\n", dependency.DisplayName)
	fmt.Printf("   Required by: %s\n", sourceID)
	fmt.Printf("   Description: %s\n", dependency.Description)
	if dependency.AlternativeSource != "" {
		fmt.Printf("   Alternative: %s\n", dependency.AlternativeSource)
	}
	if dependency.WhyNeeded != "" {
		fmt.Printf("   Why needed: %s\n", dependency.WhyNeeded)
	}
	fmt.Println()

	if dependency.AutoInstallCommand != "" {
		fmt.Printf("This dependency can be installed automatically.\n")
		fmt.Printf("Would you like to install %s now? [y/N/cancel] ", dependency.DisplayName)
	} else {
		fmt.Printf("To install %s:\n", dependency.DisplayName)
		fmt.Printf("  Visit: %s\n\n", dependency.InstallURL)
		fmt.Printf("Would you like to continue without %s? [y/N/cancel] ", dependency.DisplayName)
	}

	response, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return sync.DependencyDecisionCancel, nil
	}

	switch strings.ToLower(strings.TrimSpace(response)) {
	case affirmativeShort, affirmativeLong:
		if dependency.AutoInstallCommand != "" {
			return sync.DependencyDecisionInstall, nil
		}
		return sync.DependencyDecisionSkip, nil
	case "n", "no", "":
		return sync.DependencyDecisionSkip, nil
	case "c", "cancel":
		return sync.DependencyDecisionCancel, nil
	default:
		return 0, &pkgerrors.ValidationError{
			Field:   "dependencyDecision",
			Value:   response,
			Message: "expected yes, no, or cancel",
		}
	}
}
