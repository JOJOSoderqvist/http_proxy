package proxy

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	defaultCACertPath = "ca.crt"
	defaultCAKeyPath  = "ca.key"
	rsaBits           = 2048
	certValidity      = 365 * 24 * time.Hour
)

type CertManager struct {
	caCert        *x509.Certificate
	caPrivateKey  crypto.PrivateKey
	hostCertCache map[string]*tls.Certificate
	cacheMutex    sync.Mutex
}

func NewCertManager(caCertPath, caKeyPath string) (*CertManager, error) {
	cm := &CertManager{
		hostCertCache: make(map[string]*tls.Certificate),
	}

	err := cm.loadOrGenerateCA(caCertPath, caKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize CA: %w", err)
	}

	log.Println("CertManager initialized successfully.")
	return cm, nil
}

func (cm *CertManager) loadOrGenerateCA(certPath, keyPath string) error {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate file %s: %w", certPath, err)
	}
	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read CA private key file %s: %w", keyPath, err)
	}

	tlsCert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("failed to parse CA key pair from %s and %s: %w", certPath, keyPath, err)
	}

	certBlock, _ := pem.Decode(certPEM)
	if certBlock == nil || certBlock.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to decode PEM certificate from %s", certPath)
	}
	parsedCert, err := x509.ParseCertificate(certBlock.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate from %s: %w", certPath, err)
	}

	cm.caCert = parsedCert
	cm.caPrivateKey = tlsCert.PrivateKey
	log.Println("Successfully loaded existing CA.")
	return nil

}

func (cm *CertManager) GetCertificate(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	hostname := hello.ServerName
	if hostname == "" {

		return nil, errors.New("client did not provide SNI")
	}

	if strings.Contains(hostname, ":") {
		var err error
		hostname, _, err = net.SplitHostPort(hostname)
		if err != nil {
			return nil, fmt.Errorf("invalid SNI format: %s", hello.ServerName)
		}
	}

	cm.cacheMutex.Lock()
	defer cm.cacheMutex.Unlock()

	if cert, ok := cm.hostCertCache[hostname]; ok {
		log.Printf("Using cached certificate for %s", hostname)
		return cert, nil
	}

	log.Printf("Generating new certificate for %s", hostname)

	hostCert, err := cm.generateHostCert(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to generate certificate for %s: %w", hostname, err)
	}

	cm.hostCertCache[hostname] = hostCert
	return hostCert, nil
}

func (cm *CertManager) generateHostCert(hostname string) (*tls.Certificate, error) {
	hostPrivateKey, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key for %s: %w", hostname, err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128) // Max 128 bits
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, fmt.Errorf("failed to generate serial number for %s: %w", hostname, err)
	}

	now := time.Now()
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"KGI"},
			CommonName:   hostname,
		},
		NotBefore: now.Add(-1 * time.Minute),
		NotAfter:  now.Add(certValidity),

		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},

		BasicConstraintsValid: false,
		IsCA:                  false,

		DNSNames: []string{hostname},
	}

	if ip := net.ParseIP(hostname); ip != nil {
		template.IPAddresses = append(template.IPAddresses, ip)
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, cm.caCert, hostPrivateKey.Public(), cm.caPrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate for %s: %w", hostname, err)
	}

	tlsCert := &tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  hostPrivateKey,
		Leaf:        &template,
	}

	return tlsCert, nil
}
