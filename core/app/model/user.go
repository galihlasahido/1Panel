package model

// User is an additional login account managed by the superadmin (the
// settings-based UserName remains the bootstrap superadmin and is NOT
// a row here). RBAC v1:
//   - Menus: JSON []string of allowed top-level menu keys. "*" (or a
//     list containing "*") = all menus.
//   - Nodes: JSON []string of allowed node names incl. "local". "*" =
//     all nodes.
//
// Password is stored reversibly encrypted via libpanel/encrypt
// (StringEncrypt) to match how the settings admin password is stored,
// so the existing RSA+AES login flow can decrypt-and-compare uniformly.
type User struct {
	BaseModel
	Name        string `gorm:"uniqueIndex;not null" json:"name"`
	Password    string `gorm:"not null" json:"-"`
	Status      string `gorm:"not null;default:Enable" json:"status"`
	Language    string `gorm:"not null;default:en" json:"language"`
	Menus       string `gorm:"type:text" json:"-"`
	Nodes       string `gorm:"type:text" json:"-"`
	Description string `json:"description"`
}

func (User) TableName() string { return "users" }

const (
	UserStatusEnable  = "Enable"
	UserStatusDisable = "Disable"
)
