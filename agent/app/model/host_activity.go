package model

import "time"

// HostActivity is one host-level security event (SSH login/failure,
// Fail2Ban ban/unban, file-integrity change). Fingerprint dedupes so
// repeated collector ticks don't double-record the same event.
type HostActivity struct {
	BaseModel
	EventTime   time.Time `json:"eventTime"`
	Kind        string    `gorm:"index" json:"kind"`
	Actor       string    `json:"actor"`
	Source      string    `json:"source"`
	Target      string    `json:"target"`
	Detail      string    `json:"detail"`
	Severity    string    `gorm:"index" json:"severity"`
	Fingerprint string    `gorm:"uniqueIndex" json:"-"`
}

// FileIntegrityBaseline is the last-known-good hash of a watched path.
type FileIntegrityBaseline struct {
	BaseModel
	Path string `gorm:"uniqueIndex" json:"path"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}
