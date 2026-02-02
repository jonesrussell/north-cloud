package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	// rsaKeySize is the size of RSA keys generated for certificates.
	rsaKeySize = 2048
	// caCertValidYears is how long the CA certificate is valid.
	caCertValidYears = 10
	// leafCertValidDays is how long leaf certificates are valid.
	leafCertValidDays = 365
	// serialNumberBits is the bit size for certificate serial numbers.
	serialNumberBits = 128
)

// CertManager handles TLS certificate generation and caching.
type CertManager struct {
	caCert    *x509.Certificate
	caKey     *rsa.PrivateKey
	caCertPEM []byte
	certsDir  string
	certCache map[string]*tls.Certificate
	cacheMu   sync.RWMutex
}

// NewCertManager creates a new certificate manager.
// It loads existing CA cert/key or generates new ones.
func NewCertManager(certsDir string) (*CertManager, error) {
	cm := &CertManager{
		certsDir:  certsDir,
		certCache: make(map[string]*tls.Certificate),
	}

	if mkdirErr := os.MkdirAll(certsDir, 0o750); mkdirErr != nil {
		return nil, fmt.Errorf("failed to create certs directory: %w", mkdirErr)
	}

	caCertPath := filepath.Join(certsDir, "ca.crt")
	caKeyPath := filepath.Join(certsDir, "ca.key")

	// Try to load existing CA
	if loadErr := cm.loadCA(caCertPath, caKeyPath); loadErr != nil {
		// Generate new CA
		if genErr := cm.generateCA(caCertPath, caKeyPath); genErr != nil {
			return nil, fmt.Errorf("failed to generate CA: %w", genErr)
		}
		fmt.Printf("Generated new CA certificate at %s\n", caCertPath)
		fmt.Println("Add this CA to your system/browser trust store for HTTPS proxying")
	}

	return cm, nil
}

// CACertPEM returns the CA certificate in PEM format.
func (cm *CertManager) CACertPEM() []byte {
	return cm.caCertPEM
}

func (cm *CertManager) loadCA(certPath, keyPath string) error {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return err
	}

	keyPEM, readErr := os.ReadFile(keyPath)
	if readErr != nil {
		return readErr
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil {
		return errors.New("failed to decode CA certificate PEM")
	}

	cert, parseErr := x509.ParseCertificate(certBlock.Bytes)
	if parseErr != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", parseErr)
	}

	keyBlock, _ := pem.Decode(keyPEM)
	if keyBlock == nil {
		return errors.New("failed to decode CA key PEM")
	}

	key, keyParseErr := x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if keyParseErr != nil {
		return fmt.Errorf("failed to parse CA key: %w", keyParseErr)
	}

	cm.caCert = cert
	cm.caKey = key
	cm.caCertPEM = certPEM

	return nil
}

func (cm *CertManager) generateCA(certPath, keyPath string) error {
	// Generate private key
	key, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return fmt.Errorf("failed to generate CA key: %w", err)
	}

	// Create certificate template
	serialNumber, serialErr := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), serialNumberBits))
	if serialErr != nil {
		return fmt.Errorf("failed to generate serial number: %w", serialErr)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"North Cloud HTTP Proxy"},
			CommonName:   "North Cloud Proxy CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(caCertValidYears, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            0,
	}

	// Self-sign the certificate
	certDER, createErr := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if createErr != nil {
		return fmt.Errorf("failed to create CA certificate: %w", createErr)
	}

	// Parse back to get x509.Certificate
	cert, parseErr := x509.ParseCertificate(certDER)
	if parseErr != nil {
		return fmt.Errorf("failed to parse CA certificate: %w", parseErr)
	}

	// Encode to PEM
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})

	// Write files
	if writeCertErr := os.WriteFile(certPath, certPEM, 0o600); writeCertErr != nil {
		return fmt.Errorf("failed to write CA certificate: %w", writeCertErr)
	}
	if writeKeyErr := os.WriteFile(keyPath, keyPEM, 0o600); writeKeyErr != nil {
		return fmt.Errorf("failed to write CA key: %w", writeKeyErr)
	}

	cm.caCert = cert
	cm.caKey = key
	cm.caCertPEM = certPEM

	return nil
}

// GetCertificate returns a TLS certificate for the given domain.
// It uses cached certificates when available or generates new ones.
func (cm *CertManager) GetCertificate(domain string) (*tls.Certificate, error) {
	// Check cache first
	cm.cacheMu.RLock()
	if cert, ok := cm.certCache[domain]; ok {
		cm.cacheMu.RUnlock()
		return cert, nil
	}
	cm.cacheMu.RUnlock()

	// Generate new certificate
	cert, err := cm.generateLeafCert(domain)
	if err != nil {
		return nil, err
	}

	// Cache it
	cm.cacheMu.Lock()
	cm.certCache[domain] = cert
	cm.cacheMu.Unlock()

	return cert, nil
}

func (cm *CertManager) generateLeafCert(domain string) (*tls.Certificate, error) {
	// Generate private key
	key, err := rsa.GenerateKey(rand.Reader, rsaKeySize)
	if err != nil {
		return nil, fmt.Errorf("failed to generate leaf key: %w", err)
	}

	serialNumber, serialErr := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), serialNumberBits))
	if serialErr != nil {
		return nil, fmt.Errorf("failed to generate serial number: %w", serialErr)
	}

	template := &x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"North Cloud HTTP Proxy"},
			CommonName:   domain,
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().AddDate(0, 0, leafCertValidDays),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    []string{domain},
	}

	// Sign with CA
	certDER, createErr := x509.CreateCertificate(rand.Reader, template, cm.caCert, &key.PublicKey, cm.caKey)
	if createErr != nil {
		return nil, fmt.Errorf("failed to create leaf certificate: %w", createErr)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{certDER, cm.caCert.Raw},
		PrivateKey:  key,
	}

	return tlsCert, nil
}

// TLSConfigForClient returns a TLS config that generates certificates on demand.
func (cm *CertManager) TLSConfigForClient() *tls.Config {
	return &tls.Config{
		GetCertificate: func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
			return cm.GetCertificate(hello.ServerName)
		},
		MinVersion: tls.VersionTLS12,
	}
}
