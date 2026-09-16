package status

import "time"

// Publication summarizes verified run evidence without copying source or review lists.
type Publication struct {
	RunID            string
	ReceiptChecksum  string
	SourceCommit     string
	GenerationID     string
	CompletedAt      time.Time
	FreshAcquisition bool
	SourceCount      int
	ReviewCount      int
}
