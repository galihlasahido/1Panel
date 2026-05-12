// Package encrypt wires core's globals (CONF, DB, LOG) into libpanel's
// generic encrypt package via the KeyProvider + Logger setters. Must be
// called once during application init AFTER db.Init() so the fallback
// path can read the setting table; the file-based path works earlier.
package encrypt

import (
	"github.com/1Panel-dev/1Panel/core/app/model"
	"github.com/1Panel-dev/1Panel/core/global"
	libencrypt "github.com/1Panel-dev/1Panel/libpanel/encrypt"
)

// Init registers core's key resolution policy with libpanel/encrypt.
// Priority order on each call:
//  1. /etc/1panel/.encrypt_key (mode 0600) — preferred, lives outside the DB
//  2. global.CONF.Base.EncryptKey (process cache)
//  3. setting table EncryptKey row (one-time read; cached into 2 above)
//
// After loading from a slower source the value is best-effort written
// to the file so future boots prefer the file. This makes a stolen DB
// snapshot insufficient to decrypt secrets at rest.
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
			// db.Init() hasn't run yet; caller is too early.
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
