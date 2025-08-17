package embedded

import (
	"fmt"

	"github.com/agentstation/starmap/pkg/catalogs"
)

func PrintCatalog(catalog catalogs.Catalog) {
	fmt.Println("Catalog:")
	fmt.Println("  Providers:")
	fmt.Println("    -", catalog.Providers())
	fmt.Println("  Authors:")
	fmt.Println("    -", catalog.Authors())
}

