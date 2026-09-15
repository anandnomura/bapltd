package main

import (
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"bap-controlplane/internal/dashboardui"
	"bap-controlplane/internal/tlsutil"
)

func main() {
	port := flag.Int("port", 8444, "HTTPS port for the standalone dashboard")
	upstream := flag.String("control-plane", "https://localhost:8443", "BAP control-plane URL")
	caPath := flag.String("ca-cert", os.Getenv("BAP_CA_CERT"), "CA certificate used to verify the control plane")
	certPath := flag.String("tls-cert", "", "Dashboard HTTPS certificate")
	keyPath := flag.String("tls-key", "", "Dashboard HTTPS private key")
	clientCertPath := flag.String("client-cert", "", "Optional mTLS client certificate presented to the control plane")
	clientKeyPath := flag.String("client-key", "", "Optional mTLS client private key")
	allowRemoteAdmin := flag.Bool("allow-remote-admin", false, "Allow remote dashboard clients to invoke administrative APIs")
	flag.Parse()

	if (*certPath == "") != (*keyPath == "") {
		log.Fatal("both -tls-cert and -tls-key are required")
	}
	if (*clientCertPath == "") != (*clientKeyPath == "") {
		log.Fatal("both -client-cert and -client-key are required for mTLS")
	}
	target, err := url.Parse(*upstream)
	if err != nil || (target.Scheme != "https" && target.Scheme != "http") || target.Host == "" {
		log.Fatalf("invalid control-plane URL %q", *upstream)
	}

	roots, err := x509.SystemCertPool()
	if err != nil || roots == nil {
		roots = x509.NewCertPool()
	}
	if *caPath == "" {
		*caPath = firstExisting("bap-root-ca.crt", "../bap-root-ca.crt", "../../bap-root-ca.crt")
	}
	if *caPath != "" {
		pemBytes, err := os.ReadFile(*caPath)
		if err != nil || !roots.AppendCertsFromPEM(pemBytes) {
			log.Fatalf("cannot load control-plane CA %q", *caPath)
		}
	}
	tlsClient := &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	if *clientCertPath != "" {
		cert, err := tls.LoadX509KeyPair(*clientCertPath, *clientKeyPath)
		if err != nil {
			log.Fatalf("cannot load mTLS client identity: %v", err)
		}
		tlsClient.Certificates = []tls.Certificate{cert}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsClient

	if *certPath == "" {
		*certPath = firstExisting("controlplane-cert.pem", "../controlplane-cert.pem", "../../controlplane-cert.pem")
		*keyPath = firstExisting("controlplane-key.pem", "../controlplane-key.pem", "../../controlplane-key.pem")
	}
	if *certPath == "" || *keyPath == "" {
		cert, certPEM, keyPEM, err := tlsutil.GenerateSelfSignedCert([]string{"localhost", "127.0.0.1", "::1"}, 365*24*time.Hour)
		if err != nil {
			log.Fatal(err)
		}
		_ = cert
		*certPath, *keyPath = "dashboard-cert.pem", "dashboard-key.pem"
		if err := tlsutil.SaveCertAndKey(*certPath, *keyPath, certPEM, keyPEM); err != nil {
			log.Fatal(err)
		}
		log.Print("generated a self-signed dashboard certificate")
	}

	addr := fmt.Sprintf(":%d", *port)
	server := &http.Server{
		Addr:         addr,
		Handler:      dashboardui.Handler(target, transport, *allowRemoteAdmin),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
		TLSConfig:    &tls.Config{MinVersion: tls.VersionTLS12},
	}
	log.Printf("[bapdashboard] HTTPS dashboard starting on https://localhost:%d/dashboard/ -> %s", *port, target)
	if err := server.ListenAndServeTLS(*certPath, *keyPath); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func firstExisting(paths ...string) string {
	for _, path := range paths {
		if info, err := os.Stat(filepath.Clean(path)); err == nil && !info.IsDir() {
			return path
		}
	}
	return ""
}
