package certgen

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strings"
	"time"
)

const (
	caDefaultValidity   = 10 * 365 * 24 * time.Hour
	certDefaultValidity = 365 * 24 * time.Hour
	renewBeforeDays     = 30
)

type CertPair struct {
	CertPEM []byte
	KeyPEM  []byte
}

func generateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
}

func GenerateCA() (*CertPair, error) {
	key, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("generate CA key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate CA serial: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "l-ui CA",
			Organization: []string{"l-ui"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(caDefaultValidity),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, fmt.Errorf("create CA cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal CA key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &CertPair{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

func IssueCert(ca *CertPair, hosts []string) (*CertPair, error) {
	caCert, err := parseCert(ca.CertPEM)
	if err != nil {
		return nil, fmt.Errorf("parse CA cert: %w", err)
	}
	caKey, err := parsePrivateKey(ca.KeyPEM)
	if err != nil {
		return nil, fmt.Errorf("parse CA key: %w", err)
	}

	key, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("generate cert key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, fmt.Errorf("generate cert serial: %w", err)
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   hosts[0],
			Organization: []string{"l-ui"},
		},
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(certDefaultValidity),
		KeyUsage:  x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			tmpl.IPAddresses = append(tmpl.IPAddresses, ip)
		} else {
			tmpl.DNSNames = append(tmpl.DNSNames, h)
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return nil, fmt.Errorf("create cert: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("marshal cert key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	return &CertPair{CertPEM: certPEM, KeyPEM: keyPEM}, nil
}

func NeedsRenewal(certPEM []byte) bool {
	cert, err := parseCert(certPEM)
	if err != nil {
		return true
	}
	return time.Now().Add(renewBeforeDays * 24 * time.Hour).After(cert.NotAfter)
}

func CertInfo(certPEM []byte) (subject, issuer, sn string, notBefore, notAfter time.Time, err error) {
	cert, err := parseCert(certPEM)
	if err != nil {
		return "", "", "", time.Time{}, time.Time{}, err
	}
	return cert.Subject.CommonName, cert.Issuer.CommonName,
		cert.SerialNumber.Text(16),
		cert.NotBefore, cert.NotAfter, nil
}

func Fingerprint(certPEM []byte) (string, error) {
	cert, err := parseCert(certPEM)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", cert.SerialNumber.Bytes()), nil
}

func parseCert(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid certificate PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parsePrivateKey(pemData []byte) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("invalid key PEM")
	}

	if block.Type == "EC PRIVATE KEY" {
		return x509.ParseECPrivateKey(block.Bytes)
	}

	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse private key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not ECDSA")
	}
	return ecKey, nil
}

func HostsForNode(address, name string) []string {
	hosts := []string{address}
	if ip := net.ParseIP(address); ip == nil {
		hosts = append(hosts, name)
	}
	if strings.HasPrefix(address, "127.") || address == "localhost" || address == "::1" {
		hosts = append(hosts, "localhost")
	}
	return hosts
}
