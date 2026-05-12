package service

import (
	"errors"
	"time"

	"github.com/1Panel-dev/1Panel/core/app/repo"
	"github.com/1Panel-dev/1Panel/core/utils/encrypt"
	"github.com/1Panel-dev/1Panel/libpanel/pki"
)

// AgentCertValidity is the lifetime of each issued slave server / client
// cert. Set deliberately long (5 years) because there is no rotation flow
// in v1; operators rotate by re-enrolling a node.
const AgentCertValidity = 5 * 365 * 24 * time.Hour

type INodePKIService interface {
	// IssueAgentCert mints a new server cert + key for a slave agent. The
	// commonName must be a valid DNS label; addr is the IP or hostname the
	// master will dial. The returned PEM strings are *not* encrypted —
	// callers are expected to encrypt them before persisting.
	IssueAgentCert(commonName, addr string) (crtPEM, keyPEM string, err error)

	// IssueMasterClientCert mints a client cert (EKU=ClientAuth) the master
	// presents when dialling slaves. Issued once and reused across all
	// outbound mTLS connections from this master.
	IssueMasterClientCert(commonName string) (crtPEM, keyPEM string, err error)

	// GetCABundle returns the master's root CA in PEM form. Slaves load this
	// as their ClientCAs pool to validate the master's client cert; the
	// master also pins it as RootCAs when verifying slave server certs.
	GetCABundle() (string, error)
}

type NodePKIService struct{}

func NewINodePKIService() INodePKIService {
	return &NodePKIService{}
}

func loadCAMaterial() (caCrt, caKey string, err error) {
	settingRepo := repo.NewISettingRepo()
	crt, err := settingRepo.GetValueByKey("MasterCACrt")
	if err != nil {
		return "", "", err
	}
	if crt == "" {
		return "", "", errors.New("master CA cert is missing; migrations may not have run")
	}
	key, err := settingRepo.GetValueByKey("MasterCAKey")
	if err != nil {
		return "", "", err
	}
	if key == "" {
		return "", "", errors.New("master CA key is missing; migrations may not have run")
	}
	caCrt, err = encrypt.StringDecrypt(crt)
	if err != nil {
		return "", "", err
	}
	caKey, err = encrypt.StringDecrypt(key)
	if err != nil {
		return "", "", err
	}
	return caCrt, caKey, nil
}

func (s *NodePKIService) IssueAgentCert(commonName, addr string) (string, string, error) {
	caCrt, caKey, err := loadCAMaterial()
	if err != nil {
		return "", "", err
	}
	return pki.IssueServerCert(caCrt, caKey, commonName, addr, AgentCertValidity)
}

func (s *NodePKIService) IssueMasterClientCert(commonName string) (string, string, error) {
	caCrt, caKey, err := loadCAMaterial()
	if err != nil {
		return "", "", err
	}
	return pki.IssueClientCert(caCrt, caKey, commonName, AgentCertValidity)
}

func (s *NodePKIService) GetCABundle() (string, error) {
	caCrt, _, err := loadCAMaterial()
	if err != nil {
		return "", err
	}
	return caCrt, nil
}
