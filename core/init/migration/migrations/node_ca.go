package migrations

import (
	"time"

	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/utils/encrypt"
	"github.com/1Panel-dev/1Panel/libpanel/pki"
	"github.com/go-gormigrate/gormigrate/v2"
	"gorm.io/gorm"
)

// InitMasterCA bootstraps the master's Certificate Authority used to sign
// per-slave server certs for mTLS between the master (core) and each
// remote 1panel-agent. The CA cert and key are stored encrypted in the
// `setting` table under keys MasterCACrt / MasterCAKey.
//
// Validity is 10 years; rotation is out of scope for v1 — operators
// reissue all slave certs by re-enrolling each node.
var InitMasterCA = &gormigrate.Migration{
	ID: "20260511-init-master-ca",
	Migrate: func(tx *gorm.DB) error {
		var existing model.Setting
		err := tx.Where("key = ?", "MasterCACrt").First(&existing).Error
		if err == nil && existing.Value != "" {
			return nil
		}

		crtPEM, keyPEM, err := pki.GenerateCA(10 * 365 * 24 * time.Hour)
		if err != nil {
			return err
		}
		encCrt, err := encrypt.StringEncrypt(crtPEM)
		if err != nil {
			return err
		}
		encKey, err := encrypt.StringEncrypt(keyPEM)
		if err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "MasterCACrt", Value: encCrt}).Error; err != nil {
			return err
		}
		if err := tx.Create(&model.Setting{Key: "MasterCAKey", Value: encKey}).Error; err != nil {
			return err
		}
		return nil
	},
}
