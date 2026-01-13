package repo

type SourceStatus string

const (
	StatusUnknown     SourceStatus = "unknown"
	StatusReachable   SourceStatus = "ok"
	StatusUnreachable SourceStatus = "error"
	StatusUnsupported SourceStatus = "unsupported"
)

type Source struct {
	Manager   string       `json:"manager" cbor:"0,keyasint"`
	RepoID    string       `json:"repo_id" cbor:"1,keyasint"`
	Name      string       `json:"name,omitempty" cbor:"2,keyasint,omitempty"`
	URL       string       `json:"url" cbor:"3,keyasint"`
	Enabled   bool         `json:"enabled" cbor:"4,keyasint"`
	Status    SourceStatus `json:"status,omitempty" cbor:"5,keyasint,omitempty"`
	Error     string       `json:"error,omitempty" cbor:"6,keyasint,omitempty"`
	CheckedAt int64        `json:"checked_at,omitempty" cbor:"7,keyasint,omitempty"`
}
