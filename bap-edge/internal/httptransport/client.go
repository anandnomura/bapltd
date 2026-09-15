package httptransport

import (
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"net/http"
	"os"
	"sync"
	"time"
)

//go:embed embedded_ca.crt
var embeddedCACert []byte

// New shares the BAP_CA_CERT trust setting across every request in a process.
// It loads the embedded BAP Root CA, system certificates, and optional runtime BAP_CA_CERT.
func New(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: &caTransport{}}
}

type caTransport struct {
	once      sync.Once
	transport *http.Transport
	err       error
}

func (t *caTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.once.Do(func() {
		t.transport = http.DefaultTransport.(*http.Transport).Clone()

		roots, err := x509.SystemCertPool()
		if err != nil || roots == nil {
			roots = x509.NewCertPool()
		}

		// 1. Append the compiled-in BAP Root CA (packaged inside binary)
		if len(embeddedCACert) > 0 {
			roots.AppendCertsFromPEM(embeddedCACert)
		}

		// 2. Also check runtime BAP_CA_CERT or local files if provided
		path := os.Getenv("BAP_CA_CERT")
		if path == "" {
			for _, cand := range []string{"bap-root-ca.crt", "controlplane-cert.pem", "../bap-root-ca.crt", "../controlplane-cert.pem", "../../controlplane-cert.pem"} {
				if _, err := os.Stat(cand); err == nil {
					path = cand
					break
				}
			}
		}
		if path != "" {
			if pem, err := os.ReadFile(path); err == nil {
				roots.AppendCertsFromPEM(pem)
			}
		}

		t.transport.TLSClientConfig = &tls.Config{
			RootCAs:    roots,
			MinVersion: tls.VersionTLS12,
		}
	})
	if t.err != nil {
		return nil, t.err
	}
	return t.transport.RoundTrip(r)
}
