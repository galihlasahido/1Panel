//go:build !xpack && !xpackee

package xpack

import (
	"crypto/tls"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/1Panel-dev/1Panel/agent/app/dto"
	"github.com/1Panel-dev/1Panel/agent/app/model"
	"github.com/1Panel-dev/1Panel/agent/buserr"
	"github.com/1Panel-dev/1Panel/agent/global"
	"github.com/1Panel-dev/1Panel/agent/utils/common"
	"github.com/gin-gonic/gin"
)

func RemoveTamper(website string) {}

func StartClam(startClam *model.Clam, isUpdate bool) (int, error) {
	return 0, buserr.New("ErrXpackNotFound")
}

// slaveBootstrapDir is where the installer (run by the master during SSH-push
// enrollment) drops the per-slave TLS material. When present at first boot,
// LoadNodeInfo switches the agent into slave mode so the listening socket
// becomes mTLS-on-TCP instead of a unix socket.
const slaveBootstrapDir = "/etc/1panel/bootstrap"

func LoadNodeInfo(isBase bool) (model.NodeInfo, error) {
	var info model.NodeInfo
	info.BaseDir = common.LoadParams("BASE_DIR")
	info.Version = common.LoadParams("ORIGINAL_VERSION")

	// Slave mode: installer wrote certs + a NODE_SCOPE=slave marker file
	// to slaveBootstrapDir. Read them, set IsMaster=false, and return
	// NodeInfo populated so the InitSetting migration persists ServerCrt /
	// ServerKey / RootCrt to the agent's setting table.
	if bs, ok := readSlaveBootstrap(); ok {
		info.Scope = "slave"
		info.NodePort = bs.Port
		info.ServerCrt = bs.ServerCrt
		info.ServerKey = bs.ServerKey
		info.RootCrt = bs.RootCrt
		global.IsMaster = false
		// Bind this agent to the master by writing the ProxyID secret
		// the certificate.go middleware reads on every proxied request.
		if bs.ProxyID != "" {
			_ = os.WriteFile("/etc/1panel/.nodeProxyID", []byte(bs.ProxyID), 0o600)
		}
		return info, nil
	}

	info.Scope = "master"
	global.IsMaster = true
	return info, nil
}

type slaveBootstrap struct {
	Port      uint
	ProxyID   string
	ServerCrt string
	ServerKey string
	RootCrt   string
}

func readSlaveBootstrap() (slaveBootstrap, bool) {
	scope, err := os.ReadFile(filepath.Join(slaveBootstrapDir, "scope"))
	if err != nil || strings.TrimSpace(string(scope)) != "slave" {
		return slaveBootstrap{}, false
	}
	var bs slaveBootstrap
	if data, err := os.ReadFile(filepath.Join(slaveBootstrapDir, "port")); err == nil {
		if n, perr := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32); perr == nil {
			bs.Port = uint(n)
		}
	}
	if bs.Port == 0 {
		bs.Port = 9999
	}
	if data, err := os.ReadFile(filepath.Join(slaveBootstrapDir, "proxy_id")); err == nil {
		bs.ProxyID = strings.TrimSpace(string(data))
	}
	if data, err := os.ReadFile(filepath.Join(slaveBootstrapDir, "server.crt")); err == nil {
		bs.ServerCrt = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(slaveBootstrapDir, "server.key")); err == nil {
		bs.ServerKey = string(data)
	}
	if data, err := os.ReadFile(filepath.Join(slaveBootstrapDir, "root.crt")); err == nil {
		bs.RootCrt = string(data)
	}
	if bs.ServerCrt == "" || bs.ServerKey == "" {
		return slaveBootstrap{}, false
	}
	return bs, true
}

func GetImagePrefix() string {
	return ""
}

func IsUseCustomApp() bool {
	return false
}

func IsXpack() bool {
	return false
}

func CreateTaskScanSMSAlertLog(alert dto.AlertDTO, alertType string, create dto.AlertLogCreate, pushAlert dto.PushAlert, method string) error {
	return nil
}

func CreateSMSAlertLog(alertType string, info dto.AlertDTO, create dto.AlertLogCreate, project string, params []dto.Param, method string) error {
	return nil
}

func CreateTaskScanWebhookAlertLog(alert dto.AlertDTO, alertType string, create dto.AlertLogCreate, pushAlert dto.PushAlert, method string, transport *http.Transport, agentInfo *dto.AgentInfo) error {
	return nil
}

func CreateWebhookAlertLog(alertType string, info dto.AlertDTO, create dto.AlertLogCreate, project string, params []dto.Param, method string, transport *http.Transport, agentInfo *dto.AgentInfo) error {
	return nil
}

func GetLicenseErrorAlert() (uint, error) {
	return 0, nil
}

func GetNodeErrorAlert() (uint, error) {
	return 0, nil
}

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

func ValidateCertificate(c *gin.Context) bool {
	return true
}

func PushSSLToNode(websiteSSL *model.WebsiteSSL) error {
	return nil
}

func GetAgentInfo() (*dto.AgentInfo, error) {
	return nil, nil
}
