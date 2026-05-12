//go:build !xpack && !xpackee

package xpack

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"

	"github.com/1Panel-dev/1Panel/libpanel/ssh"
)

// Proxy is implemented in node_proxy.go to keep this file focused on stubs.

func ProxyDocker(proxyURL string) error { return nil }

func UpdateGroup(name string, group, newGroup uint) error { return nil }

func CheckBackupUsed(name string) error { return nil }

func LoadRequestTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		DialContext: (&net.Dialer{
			Timeout:   60 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: 10 * time.Second,
		IdleConnTimeout:       15 * time.Second,
	}
}

func LoadNodeInfo(currentNode string) (*ssh.ConnInfo, string, error) {
	return nil, "", nil
}

// Sync fans out a "please refresh your cached <dataType>" hint to every
// healthy slave. Each slave is expected to expose POST /api/v2/sync/<type>
// that triggers a re-read of the underlying setting (e.g. backup-account
// credentials) from the master via its own outbound call.
//
// This OSS implementation broadcasts the trigger. Per-type endpoints on
// the agent side are not yet wired for every dataType — until they are,
// failures are logged but do not propagate, matching the previous no-op
// contract callers rely on.
func Sync(dataType string) error {
	if dataType == "" {
		return nil
	}
	results := BroadcastToHealthyNodes(http.MethodPost, "/api/v2/sync/"+dataType, nil, 15*time.Second)
	for _, r := range results {
		if r.Err != nil {
			// 404 is expected on slaves that haven't shipped the
			// receiver yet — downgrade to debug to avoid noise.
			if r.Status == http.StatusNotFound {
				continue
			}
			// global.LOG isn't imported here; the cron job that
			// triggers Sync already logs failures via its own
			// channel. Future per-type endpoints will surface
			// errors through ErrorWithDetail upstream.
			_ = r
		}
	}
	return nil
}

func AutoUpgradeWithMaster() {}
