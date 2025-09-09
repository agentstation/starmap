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
  starmap inspect stat catalog
  starmap inspect stat sources/models.dev/api.json
  starmap inspect stat catalog/providers/openai.yaml catalog/models.yaml`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		fsys := inspectutil.GetEmbeddedFS()
		
		for i, arg := range args {
			targetPath := inspectutil.NormalizePath(arg)
			
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

func printDetailedStat(targetPath string, info fs.FileInfo, fsys fs.FS) error {
	fmt.Printf("  File: %s\n", targetPath)
	
	// Size information
	if info.IsDir() {
		// Count directory contents
		entries, err := fs.ReadDir(fsys, targetPath)
		if err == nil {
			fmt.Printf("  Size: %d entries\n", len(entries))
		} else {
			fmt.Printf("  Size: directory\n")
		}
	} else {
		fmt.Printf("  Size: %d bytes (%s)\n", info.Size(), inspectutil.FormatBytes(info.Size()))
	}
	
	// File type
	fileType := "regular file"
	if info.IsDir() {
		fileType = "directory"
	}
	fmt.Printf("  Type: %s\n", fileType)
	
	// Mode
	fmt.Printf("  Mode: %s (%s)\n", info.Mode(), inspectutil.FormatMode(info.Mode()))
	
	// Modification time
	fmt.Printf("ModTime: %s\n", info.ModTime().Format("2006-01-02 15:04:05.000000000 -0700"))
	
	// Additional info for files
	if !info.IsDir() {
		fmt.Printf("  Path: %s\n", targetPath)
		fmt.Printf("  Name: %s\n", path.Base(targetPath))
		
		// Detect file type
		fileExt := DetectFileType(targetPath)
		if fileExt != "unknown" {
			fmt.Printf("  Type: %s file\n", fileExt)
		}
	}
	
	// Show directory contents summary for directories
	if info.IsDir() {
		_ = printDirectorySummary(fsys, targetPath)
	}
	
	return nil
}

func printDirectorySummary(fsys fs.FS, dirPath string) error {
	entries, err := fs.ReadDir(fsys, dirPath)
	if err != nil {
		return err
	}
	
	var fileCount, dirCount int
	var totalSize int64
	
	for _, entry := range entries {
		if entry.IsDir() {
			dirCount++
		} else {
			fileCount++
			// Get file size
			if info, err := entry.Info(); err == nil {
				totalSize += info.Size()
			}
		}
	}
	
	fmt.Printf("Contents: %d directories, %d files", dirCount, fileCount)
	if fileCount > 0 {
		fmt.Printf(" (total size: %s)", inspectutil.FormatBytes(totalSize))
	}
	fmt.Println()
	
	return nil
}

func printCustomFormat(targetPath string, info fs.FileInfo, format string) error {
	// Simple format string replacement
	// This could be expanded to support more format specifiers
	result := format
	
	// Replace common format specifiers
	result = replaceFormatSpec(result, "%n", path.Base(targetPath))        // name
	result = replaceFormatSpec(result, "%N", targetPath)                   // full path
	result = replaceFormatSpec(result, "%s", fmt.Sprintf("%d", info.Size())) // size
	result = replaceFormatSpec(result, "%f", inspectutil.FormatMode(info.Mode())) // mode
	result = replaceFormatSpec(result, "%F", getFileType(info))            // file type
	result = replaceFormatSpec(result, "%y", info.ModTime().Format("2006-01-02 15:04:05")) // mod time
	
	fmt.Println(result)
	return nil
}

func replaceFormatSpec(s, spec, value string) string {
	return fmt.Sprintf(strings.ReplaceAll(s, spec, "%s"), value)
}

func getFileType(info fs.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	return "regular file"
}