package inspect

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"

	"github.com/spf13/cobra"

	inspectutil "github.com/agentstation/starmap/internal/cmd/inspect"
)

var (
	lsLong      bool
	lsHuman     bool  
	lsAll       bool
	lsRecursive bool
)

// LsCmd represents the ls command for inspecting embedded filesystem.
var LsCmd = &cobra.Command{
	Use:   "ls [path]",
	Short: "List embedded files and directories",
	Long: `List files and directories in the embedded filesystem.

Similar to the Unix ls command, this shows the contents of embedded
directories and files. By default, shows files in the root directory.

Examples:
  starmap inspect ls                      # List root directory
  starmap inspect ls catalog              # List catalog directory
  starmap inspect ls -l catalog/providers # Long format listing
  starmap inspect ls -lh sources          # Long format with human-readable sizes
  starmap inspect ls -lah sources         # Long, all files, human-readable sizes
  starmap inspect ls --help               # Show help (or use -?)`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		// If -h is used without -l, show help instead of using human-readable
		if lsHuman && !lsLong {
			return cmd.Help()
		}
		
		targetPath := "."
		if len(args) > 0 {
			targetPath = inspectutil.NormalizePath(args[0])
		}
		
		fsys := inspectutil.GetEmbeddedFS()
		return listPath(fsys, targetPath)
	},
}

func init() {
	// Define custom help flag ONLY for this ls subcommand to free up -h
	LsCmd.Flags().BoolP("help", "?", false, "help for ls command")
	
	// Now we can use -h for human-readable in this subcommand
	LsCmd.Flags().BoolVarP(&lsLong, "long", "l", false, "use a long listing format")
	LsCmd.Flags().BoolVarP(&lsHuman, "human-readable", "h", false, "print human readable sizes")
	LsCmd.Flags().BoolVarP(&lsAll, "all", "a", false, "do not ignore entries starting with .")
	LsCmd.Flags().BoolVarP(&lsRecursive, "recursive", "R", false, "list subdirectories recursively")
}

func listPath(fsys fs.FS, targetPath string) error {
	// Check if path exists and get its info
	info, err := fs.Stat(fsys, targetPath)
	if err != nil {
		return fmt.Errorf("cannot access '%s': %v", targetPath, err)
	}
	
	if !info.IsDir() {
		// If it's a file, just show the file
		return listFile(fsys, targetPath)
	}
	
	// It's a directory, list contents
	if lsRecursive {
		return listRecursive(fsys, targetPath)
	}
	return listDirectory(fsys, targetPath)
}

func listFile(fsys fs.FS, filePath string) error {
	fileInfo, err := inspectutil.GetFileInfoFromPath(fsys, filePath)
	if err != nil {
		return fmt.Errorf("cannot get info for '%s': %v", filePath, err)
	}
	
	if lsLong {
		printLongFormat([]*inspectutil.FileInfo{fileInfo})
	} else {
		fmt.Println(fileInfo.Name)
	}
	
	return nil
}

func listDirectory(fsys fs.FS, dirPath string) error {
	entries, err := fs.ReadDir(fsys, dirPath)
	if err != nil {
		return fmt.Errorf("cannot read directory '%s': %v", dirPath, err)
	}
	
	// Convert to FileInfo and filter
	files := make([]*inspectutil.FileInfo, 0, len(entries))
	for _, entry := range entries {
		// Skip hidden files unless -a flag is set
		if !lsAll && inspectutil.IsHidden(entry.Name()) {
			continue
		}
		
		fullPath := path.Join(dirPath, entry.Name())
		if dirPath == "." {
			fullPath = entry.Name()
		}
		
		fileInfo, err := inspectutil.GetFileInfoFromEntry(entry, fullPath, fsys)
		if err != nil {
			continue // Skip files we can't get info for
		}
		
		files = append(files, fileInfo)
	}
	
	// Sort files
	sort.Slice(files, func(i, j int) bool {
		// Directories first, then files
		if files[i].IsDir && !files[j].IsDir {
			return true
		}
		if !files[i].IsDir && files[j].IsDir {
			return false
		}
		return files[i].Name < files[j].Name
	})
	
	if lsLong {
		printLongFormat(files)
	} else {
		printShortFormat(files)
	}
	
	return nil
}

func listRecursive(fsys fs.FS, rootPath string) error {
	return fs.WalkDir(fsys, rootPath, func(currentPath string, d fs.DirEntry, err error) error {
		if err != nil {
			fmt.Fprintf(os.Stderr, "cannot access '%s': %v\n", currentPath, err)
			return nil // Continue walking
		}
		
		// Skip hidden files unless -a flag is set
		if !lsAll && inspectutil.IsHidden(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir // Skip entire hidden directory
			}
			return nil
		}
		
		if d.IsDir() && currentPath != rootPath {
			fmt.Printf("\n%s:\n", currentPath)
		}
		
		if !d.IsDir() || currentPath == rootPath {
			fileInfo, err := inspectutil.GetFileInfoFromEntry(d, currentPath, fsys)
			if err != nil {
				return nil
			}
			
			if lsLong {
				printLongFormat([]*inspectutil.FileInfo{fileInfo})
			} else {
				if d.IsDir() {
					// For recursive listing, show the directory name
					fmt.Printf("%s/\n", d.Name())
				} else {
					fmt.Println(d.Name())
				}
			}
		}
		
		return nil
	})
}

func printShortFormat(files []*inspectutil.FileInfo) {
	for _, file := range files {
		name := file.Name
		if file.IsDir {
			name += "/"
		}
		fmt.Println(name)
	}
}

func printLongFormat(files []*inspectutil.FileInfo) {
	for _, file := range files {
		mode := inspectutil.FormatMode(file.Mode)
		
		var size string
		if file.IsDir {
			size = "-"
		} else if lsHuman {
			size = inspectutil.FormatBytes(file.Size)
		} else {
			size = fmt.Sprintf("%d", file.Size)
		}
		
		time := inspectutil.FormatTime(file.ModTime)
		
		name := file.Name
		if file.IsDir {
			name += "/"
		}
		
		// Format: mode size time name
		// Align size to right in a fixed-width field
		fmt.Printf("%s %8s %s %s\n", mode, size, time, name)
	}
}