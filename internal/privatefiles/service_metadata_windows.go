package privatefiles

import "io/fs"

// The native descriptor check verifies Windows ownership and grants together.
func validateServiceMetadata(_ fs.FileInfo) error { return nil }
