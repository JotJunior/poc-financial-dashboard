package main_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTLSSmoke verifica que o servidor consegue iniciar e responder via HTTPS
// com um certificado auto-assinado (teste de desenvolvimento — task 2.5.4).
// Este teste valida apenas a camada TLS, não a lógica de negócio.
func TestTLSSmoke(t *testing.T) {
	// Gerar certificado auto-assinado temporário para o teste.
	certPEM, keyPEM := generateSelfSignedCert(t)

	tmpDir := t.TempDir()
	certPath := filepath.Join(tmpDir, "cert.pem")
	keyPath := filepath.Join(tmpDir, "key.pem")
	require.NoError(t, os.WriteFile(certPath, certPEM, 0600))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))

	// Carregar o certificado como tls.Certificate.
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	require.NoError(t, err, "carregar certificado e chave")

	// Criar servidor HTTPS de teste mínimo.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	srv := &http.Server{
		Handler: mux,
		TLSConfig: &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		},
	}

	// Ouvir em porta aleatória.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := "https://" + ln.Addr().String()

	go func() {
		_ = srv.ServeTLS(ln, "", "")
	}()
	defer srv.Close()

	// Aguardar o servidor iniciar.
	time.Sleep(50 * time.Millisecond)

	// Cliente HTTP que aceita o certificado auto-assinado (apenas para teste).
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(certPEM)
	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				RootCAs: certPool,
			},
		},
		Timeout: 5 * time.Second,
	}

	// Verificar que o servidor responde via HTTPS.
	resp, err := client.Get(addr + "/health")
	require.NoError(t, err, "servidor HTTPS deve responder sem erro de certificado")
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode, "health check deve retornar 200 via HTTPS")
	assert.Equal(t, "TLS 1.3", tls.VersionName(resp.TLS.Version), "deve negociar TLS 1.3 (ou mínimo 1.2)")

	t.Logf("TLS smoke test: servidor respondeu em %s com TLS %s", addr, tls.VersionName(resp.TLS.Version))
}

// generateSelfSignedCert gera um par cert/key ECDSA auto-assinado para testes.
// Válido por 1 hora; SAN para 127.0.0.1 e localhost.
func generateSelfSignedCert(t *testing.T) (certPEM, keyPEM []byte) {
	t.Helper()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Financial Dashboard Test"},
		},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
		DNSNames:    []string{"localhost"},
		NotBefore:   time.Now().Add(-1 * time.Minute),
		NotAfter:    time.Now().Add(1 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &priv.PublicKey, priv)
	require.NoError(t, err)

	certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})

	privDER, err := x509.MarshalECPrivateKey(priv)
	require.NoError(t, err)
	keyPEM = pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: privDER})

	return certPEM, keyPEM
}
