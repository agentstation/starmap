package status

import "time"

// RetentionStatus reports the last runtime collection pass without reading storage.
// Counts and bytes exclude backend replication and filesystem overhead.
type RetentionStatus struct {
	Enabled              bool
	Interval             time.Duration
	MaxGenerations       int
	MaxBytes             int64
	ScanEntries          int
	InputMaxBytes        int64
	AttemptedAt          time.Time
	SucceededAt          time.Time
	Health               Health
	Reason               string
	GenerationCollection string
	Generations          int
	GenerationBytes      int64
	ProtectedGenerations int
	ProtectedBytes       int64
	RemovedGenerations   int
	ScannedInputs        int
	InputBytes           int64
	RemovedInputs        int
	OverLimit            bool
}
