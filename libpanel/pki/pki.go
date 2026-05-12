package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net"
	"regexp"
	"time"
)

// hostnameLabel matches a single DNS label per RFC 1035: 1-63 chars, letters/
// digits/hyphens, must not start or end with a hyphen. CommonName for issued
// certs must satisfy this so it can safely be added as a DNS SAN.
var hostnameLabel = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9-]{0,61}[A-Za-z0-9])?$`)

func isValidHostnameLabel(s string) bool {
	return hostnameLabel.MatchString(s)
}

const (
	organization = "1Panel"
	ouCA         = "1Panel Master CA"
	ouServer     = "1Panel Agent"
	ouClient     = "1Panel Master"
)

func GenerateCA(validity time.Duration) (crtPEM, keyPEM string, err error) {
	key, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return "", "", err
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization:       []string{organization},
			OrganizationalUnit: []string{ouCA},
			CommonName:         "1Panel Master Root CA",
		},
		NotBefore:             now.Add(-1 * time.Minute),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}

	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return "", "", fmt.Errorf("self-sign CA: %w", err)
	}
	keyStr, err := encodeECKey(key)
	if err != nil {
		return "", "", err
	}
	return encodeCert(der), keyStr, nil
}

func IssueServerCert(caCrtPEM, caKeyPEM, commonName, addr string, validity time.Duration) (crtPEM, keyPEM string, err error) {
	return issueLeaf(caCrtPEM, caKeyPEM, commonName, addr, validity, leafKindServer)
}

func IssueClientCert(caCrtPEM, caKeyPEM, commonName string, validity time.Duration) (crtPEM, keyPEM string, err error) {
	return issueLeaf(caCrtPEM, caKeyPEM, commonName, "", validity, leafKindClient)
}

type leafKind int

const (
	leafKindServer leafKind = iota
	leafKindClient
)

func issueLeaf(caCrtPEM, caKeyPEM, commonName, addr string, validity time.Duration, kind leafKind) (string, string, error) {
	if !isValidHostnameLabel(commonName) {
		return "", "", fmt.Errorf("invalid commonName %q: must be a valid DNS label (RFC 1035)", commonName)
	}
	caCert, caKey, err := parseCAMaterial(caCrtPEM, caKeyPEM)
	if err != nil {
		return "", "", err
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", fmt.Errorf("generate leaf key: %w", err)
	}

	serial, err := randomSerial()
	if err != nil {
		return "", "", err
	}

	ou := ouServer
	eku := []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	if kind == leafKindClient {
		ou = ouClient
		eku = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			Organization:       []string{organization},
			OrganizationalUnit: []string{ou},
			CommonName:         commonName,
		},
		NotBefore:             now.Add(-1 * time.Minute),
		NotAfter:              now.Add(validity),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           eku,
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	if kind == leafKindServer {
		if addr == "" {
			return "", "", errors.New("addr is required for server cert")
		}
		if ip := net.ParseIP(addr); ip != nil {
			template.IPAddresses = []net.IP{ip}
		} else {
			template.DNSNames = []string{addr}
		}
		template.DNSNames = append(template.DNSNames, commonName)
	}

	der, err := x509.CreateCertificate(rand.Reader, template, caCert, &leafKey.PublicKey, caKey)
	if err != nil {
		return "", "", fmt.Errorf("sign leaf: %w", err)
	}
	keyStr, err := encodeECKey(leafKey)
	if err != nil {
		return "", "", err
	}
	return encodeCert(der), keyStr, nil
}

func parseCAMaterial(crtPEM, keyPEM string) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	crtBlock, _ := pem.Decode([]byte(crtPEM))
	if crtBlock == nil || crtBlock.Type != "CERTIFICATE" {
		return nil, nil, errors.New("invalid CA cert PEM")
	}
	cert, err := x509.ParseCertificate(crtBlock.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse CA cert: %w", err)
	}
	if !cert.IsCA {
		return nil, nil, errors.New("CA cert is not a CA")
	}

	keyBlock, _ := pem.Decode([]byte(keyPEM))
	if keyBlock == nil {
		return nil, nil, errors.New("invalid CA key PEM")
	}
	switch keyBlock.Type {
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("parse EC key: %w", err)
		}
		return cert, key, nil
	case "PRIVATE KEY":
		raw, err := x509.ParsePKCS8PrivateKey(keyBlock.Bytes)
		if err != nil {
			return nil, nil, fmt.Errorf("parse PKCS8 key: %w", err)
		}
		key, ok := raw.(*ecdsa.PrivateKey)
		if !ok {
			return nil, nil, errors.New("CA key is not ECDSA")
		}
		return cert, key, nil
	default:
		return nil, nil, fmt.Errorf("unsupported key PEM type: %s", keyBlock.Type)
	}
}

func encodeCert(der []byte) string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func encodeECKey(key *ecdsa.PrivateKey) (string, error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return "", fmt.Errorf("marshal EC key: %w", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: b})), nil
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	n, err := rand.Int(rand.Reader, limit)
	if err != nil {
		return nil, fmt.Errorf("serial number: %w", err)
	}
	return n, nil
}
