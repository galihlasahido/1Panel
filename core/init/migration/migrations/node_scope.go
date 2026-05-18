package migrations

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddNodeScopesTable creates `node_scopes` — saved fleet selectors
// (named label/status queries) used for scoped fan-out and bulk ops.
var AddNodeScopesTable = &gormigrate.Migration{
	ID: "20260518-add-node-scopes-table",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.NodeScope{})
	},
}
