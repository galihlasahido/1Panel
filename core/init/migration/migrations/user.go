package migrations

import (
	"github.com/1Panel-dev/1Panel/core/app/model"

	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddUserTable creates the RBAC users table (additional login accounts
// with per-user menu + node restrictions). The settings UserName stays
// the bootstrap superadmin and is intentionally not seeded here.
var AddUserTable = &gormigrate.Migration{
	ID: "20260517-add-user-table",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.User{})
	},
	Rollback: func(tx *gorm.DB) error {
		return tx.Migrator().DropTable(&model.User{})
	},
}
