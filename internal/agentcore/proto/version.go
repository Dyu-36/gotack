package proto

// VersionInfo represents version information about the server.
type VersionInfo struct {
	Protocol     int    `json:"protocol"`
	SourceDigest string `json:"source_digest,omitempty"`
	Version      string `json:"version"`
	Commit       string `json:"commit"`
	BuildID      string `json:"build_id"`
	GoVersion    string `json:"go_version"`
	Platform     string `json:"platform"`
}
