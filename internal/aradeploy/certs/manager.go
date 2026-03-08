package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

// Manager generates and manages local CA and wildcard certificates.
type Manager struct {
	certsDir   string
	dynamicDir string
}

// NewManager creates a new Manager that stores certs in {dataDir}/certs
// and dynamic traefik config in {dataDir}/dynamic.
func NewManager(dataDir string) *Manager {
	return &Manager{
		certsDir:   filepath.Join(dataDir, "certs"),
		dynamicDir: filepath.Join(dataDir, "dynamic"),
	}
}

// CACertPath returns the path to the CA certificate.
func (cm *Manager) CACertPath() string {
	return filepath.Join(cm.certsDir, "ca.crt")
}

// EnsureCerts generates the local CA (if missing) and a certificate covering
// the given domains (if missing, expired, or domains changed).
func (cm *Manager) EnsureCerts(domains []string) error {
	if len(domains) == 0 {
		return nil
	}

	if err := os.MkdirAll(cm.certsDir, 0o755); err != nil {
		return fmt.Errorf("creating certs directory: %w", err)
	}
	if err := os.MkdirAll(cm.dynamicDir, 0o755); err != nil {
		return fmt.Errorf("creating dynamic directory: %w", err)
	}

	caKeyPath := filepath.Join(cm.certsDir, "ca.key")
	caCrtPath := cm.CACertPath()

	if !fileExists(caKeyPath) || !fileExists(caCrtPath) {
		if err := cm.generateCA(caKeyPath, caCrtPath); err != nil {
			return fmt.Errorf("generating CA: %w", err)
		}
	}

	certKeyPath := filepath.Join(cm.certsDir, "wildcard.key")
	certCrtPath := filepath.Join(cm.certsDir, "wildcard.crt")

	needsRegen := !fileExists(certKeyPath) || !fileExists(certCrtPath)
	if !needsRegen {
		needsRegen = cm.isCertExpired(certCrtPath)
	}
	if !needsRegen {
		needsRegen = cm.certDomainsMismatch(certCrtPath, domains)
	}

	if needsRegen {
		if err := cm.generateSANCert(domains, caKeyPath, caCrtPath, certKeyPath, certCrtPath); err != nil {
			return fmt.Errorf("generating SAN cert: %w", err)
		}
	}

	tlsYml := `tls:
  stores:
    default:
      defaultCertificate:
        certFile: /certs/wildcard.crt
        keyFile: /certs/wildcard.key
  certificates:
    - certFile: /certs/wildcard.crt
      keyFile: /certs/wildcard.key
`
	if err := os.WriteFile(filepath.Join(cm.dynamicDir, "tls.yml"), []byte(tlsYml), 0o600); err != nil {
		return fmt.Errorf("writing tls.yml: %w", err)
	}

	return nil
}

func (cm *Manager) generateCA(keyPath, crtPath string) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serial, err := randomSerial()
	if err != nil {
		return err
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "aradeploy Local CA",
			Organization: []string{"aradeploy"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return err
	}

	if err := writePEM(keyPath, "EC PRIVATE KEY", key); err != nil {
		return err
	}
	return writeCertPEM(crtPath, certDER)
}

func (cm *Manager) generateSANCert(domains []string, caKeyPath, caCrtPath, keyPath, crtPath string) error {
	if len(domains) == 0 {
		return fmt.Errorf("no domains provided for SAN certificate")
	}

	caKey, err := loadECKey(caKeyPath)
	if err != nil {
		return fmt.Errorf("loading CA key: %w", err)
	}
	caCert, err := loadCert(caCrtPath)
	if err != nil {
		return fmt.Errorf("loading CA cert: %w", err)
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}

	serial, err := randomSerial()
	if err != nil {
		return err
	}

	dnsNames := make([]string, len(domains))
	copy(dnsNames, domains)

	cn := domains[0]
	if len(cn) > 64 {
		cn = cn[:64]
	}

	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cn,
			Organization: []string{"aradeploy"},
		},
		DNSNames:  dnsNames,
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:  x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, tmpl, caCert, &key.PublicKey, caKey)
	if err != nil {
		return err
	}

	if err := writePEM(keyPath, "EC PRIVATE KEY", key); err != nil {
		return err
	}
	return writeCertPEM(crtPath, certDER)
}

func (cm *Manager) certDomainsMismatch(certPath string, domains []string) bool {
	cert, err := loadCert(certPath)
	if err != nil {
		return true
	}
	certDNS := make(map[string]bool, len(cert.DNSNames))
	for _, dns := range cert.DNSNames {
		certDNS[dns] = true
	}
	for _, d := range domains {
		if !certDNS[d] {
			return true
		}
	}
	return false
}

func (cm *Manager) isCertExpired(certPath string) bool {
	cert, err := loadCert(certPath)
	if err != nil {
		return true
	}
	return time.Now().After(cert.NotAfter.Add(-30 * 24 * time.Hour))
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func writePEM(path, typ string, key *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // closed explicitly via pem.Encode error path
	return pem.Encode(f, &pem.Block{Type: typ, Bytes: der})
}

func writeCertPEM(path string, der []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // closed explicitly via pem.Encode error path
	return pem.Encode(f, &pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func loadECKey(path string) (*ecdsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	return x509.ParseECPrivateKey(block.Bytes)
}

func loadCert(path string) (*x509.Certificate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("no PEM block found in %s", path)
	}
	return x509.ParseCertificate(block.Bytes)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
