// Package ssh wires core's globals (LOG + setting table proxy config)
// into libpanel/ssh via SetLogger and SetProxyResolver. Must be called
// once during application init after db.Init() AND encrypt.Init().
package ssh

import (
	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/global"
	libencrypt "github.com/1Panel-dev/1Panel/libpanel/encrypt"
	libssh "github.com/1Panel-dev/1Panel/libpanel/ssh"
)

// Init registers core's logger + proxy-resolver with libpanel/ssh.
// Proxy config is read from the `setting` table (ProxyType, ProxyUrl,
// ProxyPort, ProxyUser, ProxyPasswd — last decrypted via libpanel/encrypt).
func Init() {
	libssh.SetLogger(global.LOG)
	libssh.SetProxyResolver(func() (*libssh.ProxyConfig, error) {
		settingRepo := repo.NewISettingRepo()
		proxyType, err := settingRepo.Get(repo.WithByKey("ProxyType"))
		if err != nil || len(proxyType.Value) == 0 {
			return nil, nil // signal "no proxy configured"
		}
		proxyUrl, _ := settingRepo.Get(repo.WithByKey("ProxyUrl"))
		port, _ := settingRepo.Get(repo.WithByKey("ProxyPort"))
		user, _ := settingRepo.Get(repo.WithByKey("ProxyUser"))
		passwd, _ := settingRepo.Get(repo.WithByKey("ProxyPasswd"))

		pass, _ := libencrypt.StringDecrypt(passwd.Value)
		return &libssh.ProxyConfig{
			Type:     proxyType.Value,
			URL:      proxyUrl.Value,
			Port:     port.Value,
			User:     user.Value,
			Password: pass,
		}, nil
	})
}
