package auth

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"strings"
)

// parsePEM parses a PEM-encoded EC private key (supports both PKCS#8 and SEC1).
func parsePEM(pemData string) (*ecdsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(strings.TrimSpace(pemData)))
	if block == nil {
		return nil, errors.New("invalid PEM block")
	}
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("unsupported key format")
	}
	ecKey, ok := keyAny.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("not an EC private key")
	}
	return ecKey, nil
}
