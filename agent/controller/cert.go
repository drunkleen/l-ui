package controller

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/drunkleen/l-ui/internal/logger"
	"github.com/gofiber/fiber/v3"
)

const (
	certFile = "agent_cert.pem"
	keyFile  = "agent_key.pem"
)

type certPushRequest struct {
	CertPEM string `json:"certPEM" binding:"required"`
	KeyPEM  string `json:"keyPEM" binding:"required"`
}
type certStatusResponse struct {
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	Serial      string `json:"serial"`
	NotBefore   int64  `json:"notBefore"`
	NotAfter    int64  `json:"notAfter"`
	Fingerprint string `json:"fingerprint"`
}

type CertController struct {
	certDir string
}

func NewCertController(certDir string) *CertController {
	return &CertController{certDir: certDir}
}

func (c *CertController) Push(cx fiber.Ctx) error {
	var req certPushRequest
	if err := cx.Bind().JSON(&req); err != nil {
		return cx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "msg": "invalid request: " + err.Error()})
	}

	if _, err := parseCertPEM([]byte(req.CertPEM)); err != nil {
		return cx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "msg": "invalid certificate PEM: " + err.Error()})
	}
	if _, err := parseKeyPEM([]byte(req.KeyPEM)); err != nil {
		return cx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "msg": "invalid key PEM: " + err.Error()})
	}

	if err := os.MkdirAll(c.certDir, 0700); err != nil {
		return cx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "msg": "create cert dir: " + err.Error()})
	}

	certPath := filepath.Join(c.certDir, certFile)
	if err := os.WriteFile(certPath, []byte(req.CertPEM), 0600); err != nil {
		return cx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "msg": "write cert: " + err.Error()})
	}

	keyPath := filepath.Join(c.certDir, keyFile)
	if err := os.WriteFile(keyPath, []byte(req.KeyPEM), 0600); err != nil {
		return cx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "msg": "write key: " + err.Error()})
	}

	logger.Infof("TLS cert written to %s, key to %s", certPath, keyPath)
	return cx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "msg": "certificate installed"})
}

func (c *CertController) Status(cx fiber.Ctx) error {
	certPath := filepath.Join(c.certDir, certFile)
	data, err := os.ReadFile(certPath)
	if err != nil {
		return cx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "obj": &certStatusResponse{}})
	}

	cert, err := parseCertPEM(data)
	if err != nil {
		return cx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "obj": &certStatusResponse{}})
	}

	sum := sha256.Sum256(cert.Raw)
	resp := &certStatusResponse{
		Subject:     cert.Subject.CommonName,
		Issuer:      cert.Issuer.CommonName,
		Serial:      cert.SerialNumber.Text(16),
		NotBefore:   cert.NotBefore.Unix(),
		NotAfter:    cert.NotAfter.Unix(),
		Fingerprint: fmt.Sprintf("%x", sum),
	}
	return cx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "obj": resp})
}

func parseCertPEM(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("invalid certificate PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func parseKeyPEM(data []byte) (any, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("invalid key PEM")
	}
	return x509.ParsePKCS8PrivateKey(block.Bytes)
}
