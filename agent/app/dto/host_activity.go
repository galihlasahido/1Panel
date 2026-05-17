package dto

type SearchHostActivity struct {
	PageInfo
	Kind     string `json:"kind"`
	Severity string `json:"severity"`
}
