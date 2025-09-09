// Package inspect provides commands for inspecting the embedded filesystem.
package inspect

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"github.com/spf13/cobra"

	inspectutil "github.com/agentstation/starmap/internal/cmd/inspect"
)

var (
	catShowFilename bool
	catNumber       bool
)

// CatCmd represents the cat command for inspecting embedded filesystem.
var CatCmd = &cobra.Command{
	Use:   "cat <file>...",
	Short: "Display embedded file contents",
	Long: `Display the contents of embedded files.

Similar to the Unix cat command, this reads and displays the contents
of one or more files from the embedded filesystem.

Examples:
  starmap inspect cat catalog/providers/openai.yaml
  starmap inspect cat sources/models.dev/api.json
  starmap inspect cat -n catalog/models.yaml       # Show line numbers`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		fsys := inspectutil.GetEmbeddedFS()
		
		for i, arg := range args {
			targetPath := inspectutil.NormalizePath(arg)
			
			if i > 0 {
				fmt.Println() // Blank line between files
			}
			
			if err := catFile(fsys, targetPath); err != nil {
				return err
			}
		}
		
		return nil
	},
}

func init() {
	CatCmd.Flags().BoolVarP(&catShowFilename, "filename", "f", false, "show filename header (auto-enabled for multiple files)")
	CatCmd.Flags().BoolVarP(&catNumber, "number", "n", false, "number all output lines")
}

func catFile(fsys fs.FS, filePath string) error {
	// Check if path exists and is a file
	info, err := fs.Stat(fsys, filePath)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %v", filePath, err)
	}
	
	if info.IsDir() {
		return fmt.Errorf("'%s' is a directory", filePath)
	}
	
	// Read file contents
	content, err := fs.ReadFile(fsys, filePath)
	if err != nil {
		return fmt.Errorf("cannot read '%s': %v", filePath, err)
	}
	
	// Show filename header if requested or multiple files
	if catShowFilename {
		fmt.Printf("==> %s <==\n", filePath)
	}
	
	// Display contents
	if catNumber {
		printWithLineNumbers(string(content))
	} else {
		fmt.Print(string(content))
		
		// Ensure output ends with newline if file doesn't
		if len(content) > 0 && content[len(content)-1] != '\n' {
			fmt.Println()
		}
	}
	
	return nil
}

func printWithLineNumbers(content string) {
	lines := strings.Split(content, "\n")
	
	// Don't number the final empty line if content ends with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	
	for i, line := range lines {
		fmt.Printf("%6d  %s\n", i+1, line)
	}
}

// DetectFileType returns a simple file type based on extension.
func DetectFileType(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".toml":
		return "toml"
	case ".md":
		return "markdown"
	case ".txt":
		return "text"
	case ".go":
		return "go"
	default:
		return "unknown"
	}
}