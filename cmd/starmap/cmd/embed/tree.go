package embed

import (
	"fmt"
	"io/fs"
	"path"
	"sort"

	"github.com/spf13/cobra"

	embedutil "github.com/agentstation/starmap/internal/cmd/embed"
)

var (
	treeMaxDepth int
	treeAll      bool
	treeSizes    bool
	treeNoIndent bool
)

// TreeCmd represents the tree command for inspecting embedded filesystem.
var TreeCmd = &cobra.Command{
	Use:   "tree [path]",
	Short: "Display embedded directory tree",
	Long: `Display the embedded filesystem as a tree structure.

Similar to the Unix tree command, this shows directories and files
in a hierarchical tree format with ASCII art.

Examples:
  starmap embed tree                  # Show full tree
  starmap embed tree catalog         # Show catalog directory tree  
  starmap embed tree -L 2            # Limit depth to 2 levels
  starmap embed tree -s catalog       # Show file sizes`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		targetPath := "."
		if len(args) > 0 {
			targetPath = embedutil.NormalizePath(args[0])
		}

		fsys := embedutil.GetEmbeddedFS()
		return showTree(fsys, targetPath)
	},
}

func init() {
	TreeCmd.Flags().IntVarP(&treeMaxDepth, "level", "L", 0, "limit depth of tree (0 = unlimited)")
	TreeCmd.Flags().BoolVarP(&treeAll, "all", "a", false, "show hidden files (files starting with .)")
	TreeCmd.Flags().BoolVarP(&treeSizes, "sizes", "s", false, "show file sizes")
	TreeCmd.Flags().BoolVar(&treeNoIndent, "no-indent", false, "don't print indentation lines")
}

// TreeNode represents a node in the directory tree structure.
type TreeNode struct {
	Name     string
	Path     string
	Size     int64
	IsDir    bool
	Children []*TreeNode
	Depth    int
}

func showTree(fsys fs.FS, rootPath string) error {
	// Build tree structure
	root, err := buildTree(fsys, rootPath, 0)
	if err != nil {
		return err
	}

	// Print tree
	if rootPath == "." {
		fmt.Println(".")
	} else {
		fmt.Println(rootPath)
	}

	printTree(root.Children, "")

	// Print summary
	dirCount, fileCount := countNodes(root)
	if dirCount > 0 || fileCount > 0 {
		fmt.Printf("\n%d directories", dirCount)
		if fileCount > 0 {
			fmt.Printf(", %d files", fileCount)
		}
		fmt.Println()
	}

	return nil
}

func buildTree(fsys fs.FS, currentPath string, depth int) (*TreeNode, error) {
	// Check if we've reached max depth
	if treeMaxDepth > 0 && depth >= treeMaxDepth {
		return nil, nil
	}

	info, err := fs.Stat(fsys, currentPath)
	if err != nil {
		return nil, err
	}

	node := &TreeNode{
		Name:  path.Base(currentPath),
		Path:  currentPath,
		IsDir: info.IsDir(),
		Size:  info.Size(),
		Depth: depth,
	}

	if currentPath == "." {
		node.Name = "."
	}

	if !info.IsDir() {
		return node, nil
	}

	// Read directory entries
	entries, err := fs.ReadDir(fsys, currentPath)
	if err != nil {
		return node, nil // Return node but no children
	}

	// Sort entries: directories first, then alphabetical
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() && !entries[j].IsDir() {
			return true
		}
		if !entries[i].IsDir() && entries[j].IsDir() {
			return false
		}
		return entries[i].Name() < entries[j].Name()
	})

	// Build children
	for _, entry := range entries {
		// Skip hidden files unless -a flag is set
		if !treeAll && embedutil.IsHidden(entry.Name()) {
			continue
		}

		childPath := path.Join(currentPath, entry.Name())
		if currentPath == "." {
			childPath = entry.Name()
		}

		child, err := buildTree(fsys, childPath, depth+1)
		if err != nil {
			continue // Skip files we can't process
		}
		if child != nil {
			node.Children = append(node.Children, child)
		}
	}

	return node, nil
}

func countNodes(node *TreeNode) (dirs, files int) {
	if node.IsDir {
		dirs = 1
	} else {
		files = 1
	}

	for _, child := range node.Children {
		childDirs, childFiles := countNodes(child)
		dirs += childDirs
		files += childFiles
	}

	return dirs, files
}
