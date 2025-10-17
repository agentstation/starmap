package embed

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	embedutil "github.com/agentstation/starmap/internal/cmd/embed"
)

// printWithLineNumbers displays content with line numbers.
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

// printShortFormat displays files in short format (names only).
func printShortFormat(files []*embedutil.FileInfo) {
	for _, file := range files {
		name := file.Name
		if file.IsDir {
			name += "/"
		}
		fmt.Println(name)
	}
}

// printLongFormat displays files in long format with details.
func printLongFormat(files []*embedutil.FileInfo) {
	for _, file := range files {
		mode := embedutil.FormatMode(file.Mode)

		var size string
		if file.IsDir {
			size = "-"
		} else if lsHuman {
			size = embedutil.FormatBytes(file.Size)
		} else {
			size = fmt.Sprintf("%d", file.Size)
		}

		time := embedutil.FormatTime(file.ModTime)

		name := file.Name
		if file.IsDir {
			name += "/"
		}

		// Format: mode size time name
		// Align size to right in a fixed-width field
		fmt.Printf("%s %8s %s %s\n", mode, size, time, name)
	}
}

// printTree displays nodes in tree format recursively.
func printTree(nodes []*TreeNode, prefix string, _ bool) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1

		// Print current node
		var connector, nextPrefix string

		if treeNoIndent {
			connector = ""
			nextPrefix = prefix
		} else if isLast {
			connector = "└── "
			nextPrefix = prefix + "    "
		} else {
			connector = "├── "
			nextPrefix = prefix + "│   "
		}

		// Format name with optional size and directory indicator
		name := node.Name
		if node.IsDir {
			name += "/"
		}

		if treeSizes && !node.IsDir {
			sizeStr := embedutil.FormatBytes(node.Size)
			name += fmt.Sprintf(" [%s]", sizeStr)
		}

		fmt.Printf("%s%s%s\n", prefix, connector, name)

		// Print children recursively
		if len(node.Children) > 0 {
			printTree(node.Children, nextPrefix, false)
		}
	}
}

// printDetailedStat displays detailed file/directory statistics.
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
		fmt.Printf("  Size: %d bytes (%s)\n", info.Size(), embedutil.FormatBytes(info.Size()))
	}

	// File type
	fileType := "regular file"
	if info.IsDir() {
		fileType = "directory"
	}
	fmt.Printf("  Type: %s\n", fileType)

	// Mode
	fmt.Printf("  Mode: %s (%s)\n", info.Mode(), embedutil.FormatMode(info.Mode()))

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

// printDirectorySummary displays a summary of directory contents.
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
		fmt.Printf(" (total size: %s)", embedutil.FormatBytes(totalSize))
	}
	fmt.Println()

	return nil
}

// printCustomFormat displays file info using a custom format string.
func printCustomFormat(targetPath string, info fs.FileInfo, format string) error {
	// Simple format string replacement
	// This could be expanded to support more format specifiers
	result := format

	// Replace common format specifiers
	result = replaceFormatSpec(result, "%n", path.Base(targetPath))                        // name
	result = replaceFormatSpec(result, "%N", targetPath)                                   // full path
	result = replaceFormatSpec(result, "%s", fmt.Sprintf("%d", info.Size()))               // size
	result = replaceFormatSpec(result, "%f", embedutil.FormatMode(info.Mode()))            // mode
	result = replaceFormatSpec(result, "%F", getFileType(info))                            // file type
	result = replaceFormatSpec(result, "%y", info.ModTime().Format("2006-01-02 15:04:05")) // mod time

	fmt.Println(result)
	return nil
}

// replaceFormatSpec replaces a format specifier in a string with a value.
func replaceFormatSpec(s, spec, value string) string {
	return fmt.Sprintf(strings.ReplaceAll(s, spec, "%s"), value)
}

// getFileType returns a simple file type description.
func getFileType(info fs.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	return "regular file"
}
