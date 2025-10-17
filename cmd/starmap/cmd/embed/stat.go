package embed

import (
	"fmt"
	"io/fs"

	"github.com/spf13/cobra"

	embedutil "github.com/agentstation/starmap/internal/cmd/embed"
)

var (
	statFormat string
)

// StatCmd represents the stat command for inspecting embedded filesystem.
var StatCmd = &cobra.Command{
	Use:   "stat <path>...",
	Short: "Display embedded file/directory status",
	Long: `Display detailed information about embedded files and directories.

Similar to the Unix stat command, this shows comprehensive information
about files and directories in the embedded filesystem including
size, permissions, and type.

Examples:
  starmap embed stat catalog
  starmap embed stat sources/models.dev/api.json
  starmap embed stat catalog/providers/openai.yaml catalog/models.yaml`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		fsys := embedutil.GetEmbeddedFS()

		for i, arg := range args {
			targetPath := embedutil.NormalizePath(arg)

			if i > 0 {
				fmt.Println() // Blank line between entries
			}

			if err := statPath(fsys, targetPath); err != nil {
				return err
			}
		}

		return nil
	},
}

func init() {
	StatCmd.Flags().StringVarP(&statFormat, "format", "c", "", "use custom format string")
}

func statPath(fsys fs.FS, targetPath string) error {
	info, err := fs.Stat(fsys, targetPath)
	if err != nil {
		return fmt.Errorf("cannot stat '%s': %v", targetPath, err)
	}

	// Use custom format if provided
	if statFormat != "" {
		return printCustomFormat(targetPath, info, statFormat)
	}

	// Default detailed format
	return printDetailedStat(targetPath, info, fsys)
}
