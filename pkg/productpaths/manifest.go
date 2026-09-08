package productpaths

// FileManifest describes file locations without claiming that the files exist.
// The host supplies the selected configuration and implementation availability.
type FileManifest struct {
	SchemaVersion          int             `json:"schema_version" yaml:"schema_version"`
	Product                Product         `json:"product" yaml:"product"`
	BuildVersion           string          `json:"build_version" yaml:"build_version"`
	DeploymentID           string          `json:"deployment_id" yaml:"deployment_id"`
	InstanceID             string          `json:"instance_id" yaml:"instance_id"`
	RelativePathBase       string          `json:"relative_path_base" yaml:"relative_path_base"`
	RelativePathBaseOrigin string          `json:"relative_path_base_origin" yaml:"relative_path_base_origin"`
	Roots                  Roots           `json:"roots" yaml:"roots"`
	Files                  []FileEntry     `json:"files" yaml:"files"`
	External               []ExternalFiles `json:"external" yaml:"external"`
	Inspection             *FileInspection `json:"inspection,omitempty" yaml:"inspection,omitempty"`
}

// FileEntry identifies a file or a bounded tree and its creation and recovery rules.
// Patterns use slash-separated relative names. A double star includes nested content.
// For a patterns entry, Policy applies to matches, not to the Location scan parent.
// Availability is available, disabled, or planned. Planned paths have no implemented writer.
type FileEntry struct {
	ID           string     `json:"id" yaml:"id"`
	Location     Path       `json:"location" yaml:"location"`
	Kind         string     `json:"kind" yaml:"kind"`
	Patterns     []string   `json:"patterns,omitempty" yaml:"patterns,omitempty"`
	Availability string     `json:"availability" yaml:"availability"`
	Creation     string     `json:"creation" yaml:"creation"`
	Recovery     string     `json:"recovery" yaml:"recovery"`
	Policy       FilePolicy `json:"policy" yaml:"policy"`
}

// FilePolicy describes required access and operator-controlled retention.
// It does not assert effective permissions or enable automatic deletion.
type FilePolicy struct {
	Selectors     []string `json:"selectors" yaml:"selectors"`
	Applicability string   `json:"applicability" yaml:"applicability"`
	Access        string   `json:"access" yaml:"access"`
	Retention     string   `json:"retention" yaml:"retention"`
	Removal       string   `json:"removal" yaml:"removal"`
}

// ExternalFiles describes destinations that operators select outside the product roots.
// The report omits destinations that the current configuration does not select.
type ExternalFiles struct {
	ID        string     `json:"id" yaml:"id"`
	Selection string     `json:"selection" yaml:"selection"`
	Recovery  string     `json:"recovery" yaml:"recovery"`
	Policy    FilePolicy `json:"policy" yaml:"policy"`
}
