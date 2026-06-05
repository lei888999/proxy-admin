package inbound

import (
	"crypto/x509"
	"encoding/pem"
	"testing"
)

func TestSelfSignedCert(t *testing.T) {
	certPEM, keyPEM, err := selfSignedCert("bing.com")
	if err != nil {
		t.Fatalf("selfSignedCert: %v", err)
	}
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		t.Fatal("cert not PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatalf("parse cert: %v", err)
	}
	found := false
	for _, n := range cert.DNSNames {
		if n == "bing.com" {
			found = true
		}
	}
	if !found {
		t.Fatalf("SAN missing bing.com: %v", cert.DNSNames)
	}
	if block, _ := pem.Decode([]byte(keyPEM)); block == nil {
		t.Fatal("key not PEM")
	}
}
