package certgen

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestGenerateCA(t *testing.T) {
	ca, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA() returned error: %v", err)
	}

	if len(ca.CertPEM) == 0 {
		t.Fatal("GenerateCA() returned empty CertPEM")
	}
	if len(ca.KeyPEM) == 0 {
		t.Fatal("GenerateCA() returned empty KeyPEM")
	}

	block, _ := pem.Decode(ca.CertPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("CertPEM is not a valid PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Parse certificate: %v", err)
	}

	if !cert.IsCA {
		t.Fatal("CA certificate should have IsCA=true")
	}
	if cert.MaxPathLen != 1 {
		t.Fatalf("CA MaxPathLen = %d, want 1", cert.MaxPathLen)
	}
	if !cert.BasicConstraintsValid {
		t.Fatal("CA BasicConstraintsValid should be true")
	}
	if cert.Subject.CommonName != "l-ui CA" {
		t.Fatalf("CA CommonName = %q, want %q", cert.Subject.CommonName, "l-ui CA")
	}
	if time.Until(cert.NotAfter) < 9*365*24*time.Hour {
		t.Fatal("CA validity should be ~10 years")
	}
}

func TestIssueCert(t *testing.T) {
	ca, err := GenerateCA()
	if err != nil {
		t.Fatalf("GenerateCA(): %v", err)
	}

	hosts := []string{"node1.example.com", "192.168.1.100", "localhost"}
	pair, err := IssueCert(ca, hosts)
	if err != nil {
		t.Fatalf("IssueCert() returned error: %v", err)
	}

	if len(pair.CertPEM) == 0 {
		t.Fatal("IssueCert() returned empty CertPEM")
	}
	if len(pair.KeyPEM) == 0 {
		t.Fatal("IssueCert() returned empty KeyPEM")
	}

	block, _ := pem.Decode(pair.CertPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		t.Fatal("CertPEM is not a valid PEM certificate")
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("Parse certificate: %v", err)
	}

	if cert.Subject.CommonName != hosts[0] {
		t.Fatalf("CommonName = %q, want %q", cert.Subject.CommonName, hosts[0])
	}
	if cert.IsCA {
		t.Fatal("Issued cert should not be a CA")
	}

	caBlock, _ := pem.Decode(ca.CertPEM)
	caCert, _ := x509.ParseCertificate(caBlock.Bytes)
	if err := cert.CheckSignatureFrom(caCert); err != nil {
		t.Fatalf("Cert signature verification against CA failed: %v", err)
	}

	foundIP := false
	foundDNS := false
	for _, ip := range cert.IPAddresses {
		if ip.String() == "192.168.1.100" {
			foundIP = true
		}
	}
	for _, dns := range cert.DNSNames {
		if dns == "node1.example.com" {
			foundDNS = true
		}
	}

	if !foundIP {
		t.Fatal("Cert should contain IP 192.168.1.100 in SANs")
	}
	if !foundDNS {
		t.Fatal("Cert should contain DNS node1.example.com in SANs")
	}
}

func TestNeedsRenewal(t *testing.T) {
	ca, _ := GenerateCA()
	hosts := []string{"test.example.com"}
	pair, _ := IssueCert(ca, hosts)

	if NeedsRenewal(pair.CertPEM) {
		t.Fatal("Freshly issued cert should not need renewal")
	}

	if NeedsRenewal([]byte("invalid")) {
		t.Log("NeedsRenewal returns true for invalid PEM (expected)")
	} else {
		t.Fatal("NeedsRenewal should return true for invalid PEM")
	}
}

func TestCertInfo(t *testing.T) {
	ca, _ := GenerateCA()
	hosts := []string{"info.example.com"}
	pair, _ := IssueCert(ca, hosts)

	subject, issuer, serial, notBefore, notAfter, err := CertInfo(pair.CertPEM)
	if err != nil {
		t.Fatalf("CertInfo() returned error: %v", err)
	}

	if subject != "info.example.com" {
		t.Fatalf("subject = %q, want %q", subject, "info.example.com")
	}
	if issuer != "l-ui CA" {
		t.Fatalf("issuer = %q, want %q", issuer, "l-ui CA")
	}
	if serial == "" {
		t.Fatal("serial should not be empty")
	}
	if notBefore.IsZero() {
		t.Fatal("notBefore should not be zero")
	}
	if notAfter.IsZero() {
		t.Fatal("notAfter should not be zero")
	}
	if notAfter.Before(notBefore) {
		t.Fatal("notAfter should be after notBefore")
	}
}

func TestFingerprint(t *testing.T) {
	ca, _ := GenerateCA()
	hosts := []string{"fp.example.com"}
	pair, _ := IssueCert(ca, hosts)

	fp, err := Fingerprint(pair.CertPEM)
	if err != nil {
		t.Fatalf("Fingerprint() returned error: %v", err)
	}
	if fp == "" {
		t.Fatal("Fingerprint should not be empty")
	}

	_, err = Fingerprint([]byte("invalid"))
	if err == nil {
		t.Fatal("Fingerprint should return error for invalid PEM")
	}
}

func TestHostsForNode(t *testing.T) {
	tests := []struct {
		address  string
		name     string
		wantHost []string
	}{
		{"192.168.1.1", "node1", []string{"192.168.1.1"}},
		{"node1.example.com", "node1", []string{"node1.example.com", "node1"}},
		{"localhost", "local", []string{"localhost", "local", "localhost"}},
		{"127.0.0.1", "local", []string{"127.0.0.1", "localhost"}},
		{"::1", "local", []string{"::1", "localhost"}},
		{"10.0.0.1", "node10", []string{"10.0.0.1"}},
	}

	for _, tt := range tests {
		t.Run(tt.address+"/"+tt.name, func(t *testing.T) {
			hosts := HostsForNode(tt.address, tt.name)
			if len(hosts) != len(tt.wantHost) {
				t.Errorf("HostsForNode(%q, %q) = %v, want %v",
					tt.address, tt.name, hosts, tt.wantHost)
				return
			}
			for i, h := range hosts {
				if h != tt.wantHost[i] {
					t.Errorf("HostsForNode(%q, %q)[%d] = %q, want %q",
						tt.address, tt.name, i, h, tt.wantHost[i])
				}
			}
		})
	}
}
