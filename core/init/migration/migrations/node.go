package migrations

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddNodeTable creates the `nodes` table used by the multi-server feature.
// One row per remote 1panel-agent managed by this master.
var AddNodeTable = &gormigrate.Migration{
	ID: "20260511-add-node-table",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.Node{})
	},
}
