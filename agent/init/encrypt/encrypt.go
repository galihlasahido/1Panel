// Package encrypt wires agent's globals into libpanel/encrypt via the
// KeyProvider + Logger setters. Must be called once during agent init
// after db.Init() so the DB fallback works.
package encrypt

import (
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/global"
	libencrypt "github.com/1Panel-dev/1Panel/libpanel/encrypt"
)

// Init registers agent's key resolution policy with libpanel/encrypt.
// Priority: file → process cache → DB setting row. See core/init/encrypt
// for the matching master implementation.
func Init() {
	libencrypt.SetKeyProvider(func() (string, error) {
		if k, ok := libencrypt.ReadKeyFile(); ok {
			if global.CONF.Base.EncryptKey != k {
				global.CONF.Base.EncryptKey = k
			}
			return k, nil
		}
		if len(global.CONF.Base.EncryptKey) > 0 {
			_ = libencrypt.WriteKeyFile(global.CONF.Base.EncryptKey)
			return global.CONF.Base.EncryptKey, nil
		}
		if global.DB == nil {
			return "", nil
		}
		var s model.Setting
		if err := global.DB.Where("key = ?", "EncryptKey").First(&s).Error; err != nil {
			return "", err
		}
		global.CONF.Base.EncryptKey = s.Value
		_ = libencrypt.WriteKeyFile(s.Value)
		return s.Value, nil
	})
	libencrypt.SetLogger(global.LOG)
}
