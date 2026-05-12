package pki

import (
	"crypto/x509"
	"encoding/pem"
	"strings"
	"testing"
	"time"
)

func TestGenerateCA(t *testing.T) {
	crtPEM, keyPEM, err := GenerateCA(10 * 365 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	if !strings.Contains(crtPEM, "BEGIN CERTIFICATE") {
		t.Fatalf("cert PEM missing header")
	}
	if !strings.Contains(keyPEM, "BEGIN EC PRIVATE KEY") {
		t.Fatalf("key PEM missing header")
	}

	block, _ := pem.Decode([]byte(crtPEM))
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	if !cert.IsCA {
		t.Fatalf("cert is not CA")
	}
	if cert.KeyUsage&x509.KeyUsageCertSign == 0 {
		t.Fatalf("CA missing CertSign usage")
	}
}

func TestIssueServerCert_IP(t *testing.T) {
	caCrt, caKey, err := GenerateCA(10 * 365 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	leafCrt, leafKey, err := IssueServerCert(caCrt, caKey, "prod-web-1", "10.0.0.5", 5*365*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	if !strings.Contains(leafKey, "BEGIN EC PRIVATE KEY") {
		t.Fatalf("leaf key PEM bad")
	}

	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM([]byte(caCrt)) {
		t.Fatalf("AppendCertsFromPEM failed")
	}

	block, _ := pem.Decode([]byte(leafCrt))
	leaf, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse leaf: %v", err)
	}

	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}); err != nil {
		t.Fatalf("verify leaf: %v", err)
	}
	if len(leaf.IPAddresses) != 1 || leaf.IPAddresses[0].String() != "10.0.0.5" {
		t.Fatalf("expected IP SAN 10.0.0.5, got %v", leaf.IPAddresses)
	}
	if leaf.Subject.CommonName != "prod-web-1" {
		t.Fatalf("CN mismatch: %s", leaf.Subject.CommonName)
	}
	foundDNS := false
	for _, dns := range leaf.DNSNames {
		if dns == "prod-web-1" {
			foundDNS = true
		}
	}
	if !foundDNS {
		t.Fatalf("expected commonName in DNS SAN, got %v", leaf.DNSNames)
	}
}

func TestIssueServerCert_DNS(t *testing.T) {
	caCrt, caKey, err := GenerateCA(10 * 365 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	leafCrt, _, err := IssueServerCert(caCrt, caKey, "edge-node", "edge.example.com", 5*365*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueServerCert: %v", err)
	}
	block, _ := pem.Decode([]byte(leafCrt))
	leaf, _ := x509.ParseCertificate(block.Bytes)
	hasAddr := false
	for _, dns := range leaf.DNSNames {
		if dns == "edge.example.com" {
			hasAddr = true
		}
	}
	if !hasAddr {
		t.Fatalf("expected DNS SAN edge.example.com, got %v", leaf.DNSNames)
	}
	if len(leaf.IPAddresses) != 0 {
		t.Fatalf("expected no IP SAN, got %v", leaf.IPAddresses)
	}
}

func TestIssueClientCert(t *testing.T) {
	caCrt, caKey, err := GenerateCA(10 * 365 * 24 * time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	clientCrt, _, err := IssueClientCert(caCrt, caKey, "master", 5*365*24*time.Hour)
	if err != nil {
		t.Fatalf("IssueClientCert: %v", err)
	}

	block, _ := pem.Decode([]byte(clientCrt))
	cert, _ := x509.ParseCertificate(block.Bytes)
	hasClientAuth := false
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			hasClientAuth = true
		}
	}
	if !hasClientAuth {
		t.Fatalf("client cert missing ClientAuth EKU")
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM([]byte(caCrt))
	if _, err := cert.Verify(x509.VerifyOptions{
		Roots:     pool,
		KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}); err != nil {
		t.Fatalf("verify client cert: %v", err)
	}
}

func TestIssueServerCert_RejectsBadCA(t *testing.T) {
	_, _, err := IssueServerCert("not a pem", "not a pem", "x", "1.2.3.4", time.Hour)
	if err == nil {
		t.Fatalf("expected error on bad CA PEM")
	}
}

func TestIssueServerCert_RejectsInvalidCommonName(t *testing.T) {
	caCrt, caKey, err := GenerateCA(time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	cases := []string{"", "-leading-hyphen", "trailing-hyphen-", "has space", "has\nnewline", strings.Repeat("a", 64)}
	for _, cn := range cases {
		if _, _, err := IssueServerCert(caCrt, caKey, cn, "1.2.3.4", time.Hour); err == nil {
			t.Fatalf("expected error for invalid commonName %q", cn)
		}
	}
}

func TestIssueServerCert_RejectsEmptyAddr(t *testing.T) {
	caCrt, caKey, err := GenerateCA(time.Hour)
	if err != nil {
		t.Fatalf("GenerateCA: %v", err)
	}
	_, _, err = IssueServerCert(caCrt, caKey, "x", "", time.Hour)
	if err == nil {
		t.Fatalf("expected error on empty addr")
	}
}
