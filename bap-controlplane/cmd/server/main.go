package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"bap-controlplane/internal/api"
	"bap-controlplane/internal/audit"
	"bap-controlplane/internal/authz"
	"bap-controlplane/internal/otc"
	"bap-controlplane/internal/policy"
	"bap-controlplane/internal/registry"
	"bap-controlplane/internal/session"
	"bap-controlplane/internal/tlsutil"
)

func main() {
	port := flag.Int("port", 8080, "Port for bapcontrolplane HTTP server")
	secretKey := flag.String("secret", "ltd-service-bounded-authority-secret-key-32b!", "HMAC signing secret for tokens")
	grantTTL := flag.Int("ttl", 30, "Default grant TTL in minutes")
	policyPath := flag.String("policy", "", "Path to authoritative Cedar policy file")
	schemaPath := flag.String("schema", "", "Path to authoritative Cedar schema file")
	trustDomain := flag.String("trust-domain", "bap.internal", "SPIFFE Trust Domain for agent workload identities")
	useTLS := flag.Bool("tls", false, "Enable HTTPS/TLS")
	autoTLS := flag.Bool("tls-auto", false, "Auto-generate self-signed TLS certificates for development")
	certPath := flag.String("tls-cert", "", "Path to TLS certificate PEM file")
	keyPath := flag.String("tls-key", "", "Path to TLS private key PEM file")
	flag.Parse()

	if envSecret := os.Getenv("BAP_SECRET_KEY"); envSecret != "" {
		*secretKey = envSecret
	}
	if *secretKey == "ltd-service-bounded-authority-secret-key-32b!" {
		log.Printf("[bapcontrolplane] WARNING: Running with default HMAC secret key. Set BAP_SECRET_KEY in production!")
	}

	if envTD := os.Getenv("BAP_TRUST_DOMAIN"); envTD != "" {
		*trustDomain = envTD
	}

	regStore := registry.NewStore(*trustDomain)
	otcStore := otc.NewStore()
	minter := authz.NewTokenMinter(*secretKey, time.Duration(*grantTTL)*time.Minute)

	resolvedPolicy := *policyPath
	if resolvedPolicy == "" {
		candidates := []string{"bap-edge/policy.cedar", "./bap-edge/policy.cedar", "../bap-edge/policy.cedar", "../ltd-agent/policy.cedar", "./policy.cedar", "../policy.cedar"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				resolvedPolicy = c
				break
			}
		}
	}
	resolvedSchema := *schemaPath
	if resolvedSchema == "" {
		candidates := []string{"bap-edge/schema.json", "./bap-edge/schema.json", "../bap-edge/schema.json", "../ltd-agent/schema.json", "./schema.json", "../schema.json"}
		for _, c := range candidates {
			if _, err := os.Stat(c); err == nil {
				resolvedSchema = c
				break
			}
		}
	}

	cedarContent, _ := os.ReadFile(resolvedPolicy)
	schemaContent, _ := os.ReadFile(resolvedSchema)
	policyStore := policy.NewStore(string(cedarContent), string(schemaContent))
	auditStore := audit.NewStore()
	sessionStore := session.NewStore()

	server := api.NewServer(regStore, otcStore, minter, policyStore, auditStore, sessionStore)

	addr := fmt.Sprintf(":%d", *port)
	proto := "http"
	if *useTLS || *autoTLS {
		proto = "https"
	}
	log.Printf("[bapcontrolplane] Central Control Plane for Bounded Authority Plane starting on %s://%s (Alias: ltd-service)", proto, addr)
	log.Printf("[bapcontrolplane] Trust Domain: %s (SPIFFE format: spiffe://%s/app/{app_id}/instance/{instance_id})", *trustDomain, *trustDomain)
	log.Printf("[bapcontrolplane] Features: Agent Registry, Multi-Instance Quotas, Binary Hash Attestation, Dynamic Policy Sync, Audit Ingestion, Session Lifecycle")

	srv := &http.Server{
		Addr:         addr,
		Handler:      server.Handler(),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	if *useTLS || *autoTLS {
		if *autoTLS || (*certPath == "" && *keyPath == "") {
			log.Printf("[bapcontrolplane] Generating self-signed TLS certificates for development...")
			tlsCert, certPEM, keyPEM, err := tlsutil.GenerateSelfSignedCert([]string{"localhost", "127.0.0.1"}, 365*24*time.Hour)
			if err != nil {
				log.Fatalf("[bapcontrolplane] Failed to generate self-signed cert: %v", err)
			}
			_ = tlsutil.SaveCertAndKey("controlplane-cert.pem", "controlplane-key.pem", certPEM, keyPEM)
			log.Printf("[bapcontrolplane] Exported dev cert to controlplane-cert.pem for client trust verification")
			srv.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{tlsCert},
			}
			if err := srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				log.Fatalf("[bapcontrolplane] Fatal TLS server error: %v", err)
			}
			return
		}

		if err := srv.ListenAndServeTLS(*certPath, *keyPath); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[bapcontrolplane] Fatal TLS server error: %v", err)
		}
		return
	}

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("[bapcontrolplane] Fatal server error: %v", err)
	}
}
