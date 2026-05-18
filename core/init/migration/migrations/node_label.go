package migrations

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// AddNodeLabelsTable creates the `node_labels` table — many-to-one
// key=value tags used for fleet scoping (env=prod, region=us, ...).
var AddNodeLabelsTable = &gormigrate.Migration{
	ID: "20260518-add-node-labels-table",
	Migrate: func(tx *gorm.DB) error {
		return tx.AutoMigrate(&model.NodeLabel{})
	},
}
