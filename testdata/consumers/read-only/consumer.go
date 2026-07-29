// Package consumer is a real external module exercising Starmap's canonical
// read-only library interface.
package consumer

import "github.com/agentstation/starmap"

// Lookup proves that ordinary consumers need only construct Starmap, retain
// its immutable catalog, and query a canonical model.
func Lookup() error {
	sm, err := starmap.New()
	if err != nil {
		return err
	}

	catalog := sm.Catalog()
	_, err = catalog.FindModel("gpt-4o")
	return err
}
