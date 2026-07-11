package starmap

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/agentstation/starmap/pkg/catalogscheduler"
	pkgsync "github.com/agentstation/starmap/pkg/sync"
)

type explicitCatalogSyncer interface {
	Sync(context.Context, ...pkgsync.Option) (*pkgsync.Result, error)
}

var _ explicitCatalogSyncer = (*Client)(nil)
var _ catalogscheduler.CurrentGenerationReader = (*Client)(nil)

func TestSchedulerOwnershipRootClientExposesSyncWithoutCadenceLifecycle(t *testing.T) {
	clientType := reflect.TypeFor[*Client]()
	for _, forbidden := range []string{"AutoUpdatesOn", "AutoUpdatesOff", "StartScheduler", "StopScheduler"} {
		if _, found := clientType.MethodByName(forbidden); found {
			t.Errorf("root Client retains deployment cadence method %s", forbidden)
		}
	}

	files, err := parser.ParseDir(token.NewFileSet(), ".", func(info fs.FileInfo) bool {
		name := info.Name()
		return strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("ParseDir root package: %v", err)
	}
	root := files["starmap"]
	if root == nil {
		t.Fatal("root starmap package was not parsed")
	}
	for filename, file := range root.Files {
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "NewTicker" {
				return true
			}
			identifier, ok := selector.X.(*ast.Ident)
			if ok && identifier.Name == "time" {
				t.Errorf("root package owns time.NewTicker cadence in %s", filename)
			}
			return true
		})
	}
}
