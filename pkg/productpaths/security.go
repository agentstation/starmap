package productpaths

// WindowsSecurity reports native ownership and a limited private DACL assessment.
// A compatible policy does not establish effective access or ancestor safety.
type WindowsSecurity struct {
	OwnerSID     string `json:"owner_sid,omitempty" yaml:"owner_sid,omitempty"`
	ProcessSID   string `json:"process_sid,omitempty" yaml:"process_sid,omitempty"`
	DACLState    string `json:"dacl_state" yaml:"dacl_state"`
	EntryCount   int    `json:"entry_count" yaml:"entry_count"`
	PolicyStatus string `json:"policy_status" yaml:"policy_status"`
	Reason       string `json:"reason,omitempty" yaml:"reason,omitempty"`
}
