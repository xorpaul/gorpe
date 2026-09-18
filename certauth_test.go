package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"golang.org/x/crypto/ocsp"
)

// testPKI holds an in-memory CA and a client certificate signed by it.
type testPKI struct {
	caKey      *ecdsa.PrivateKey
	caCert     *x509.Certificate
	clientCert *x509.Certificate
}

// newTestPKI creates a CA and a client cert (serial 42) embedding the given OCSP / CRL URLs.
// Pass empty strings to omit the respective extension.
func newTestPKI(t *testing.T, ocspURL, crlURL string) *testPKI {
	t.Helper()

	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caCertDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		t.Fatal(err)
	}

	clientKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject: pkix.Name{
			Organization: []string{"TestOrg"},
			CommonName:   "test-client",
		},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	if ocspURL != "" {
		clientTemplate.OCSPServer = []string{ocspURL}
	}
	if crlURL != "" {
		clientTemplate.CRLDistributionPoints = []string{crlURL}
	}

	clientCertDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	clientCert, err := x509.ParseCertificate(clientCertDER)
	if err != nil {
		t.Fatal(err)
	}

	return &testPKI{caKey: caKey, caCert: caCert, clientCert: clientCert}
}

// ocspResponder is an httptest.Server that serves CA-signed OCSP responses.
// Set pki after calling startOCSPResponder and before making any requests.
type ocspResponder struct {
	*httptest.Server
	pki    *testPKI
	status int
}

func startOCSPResponder(t *testing.T, status int) *ocspResponder {
	t.Helper()
	r := &ocspResponder{status: status}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		ocspReq, err := ocsp.ParseRequest(body)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		template := ocsp.Response{
			Status:       r.status,
			SerialNumber: ocspReq.SerialNumber,
			ThisUpdate:   time.Now(),
			NextUpdate:   time.Now().Add(time.Hour),
		}
		respBytes, err := ocsp.CreateResponse(r.pki.caCert, r.pki.caCert, template, r.pki.caKey)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/ocsp-response")
		w.Write(respBytes)
	}))
	t.Cleanup(r.Server.Close)
	return r
}

// buildCRL returns a DER-encoded CRL signed by pki.caKey containing revokedSerials.
// If tamper is true, the last byte of the DER is flipped to corrupt the signature.
func buildCRL(t *testing.T, pki *testPKI, revokedSerials []*big.Int, tamper bool) []byte {
	t.Helper()
	entries := make([]x509.RevocationListEntry, len(revokedSerials))
	for i, s := range revokedSerials {
		entries[i] = x509.RevocationListEntry{
			SerialNumber:   s,
			RevocationTime: time.Now().Add(-time.Minute),
		}
	}
	der, err := x509.CreateRevocationList(rand.Reader, &x509.RevocationList{
		RevokedCertificateEntries: entries,
		Number:                    big.NewInt(1),
		ThisUpdate:                time.Now().Add(-time.Minute),
		NextUpdate:                time.Now().Add(time.Hour),
	}, pki.caCert, pki.caKey)
	if err != nil {
		t.Fatal(err)
	}
	if tamper {
		der[len(der)-1] ^= 0xff
	}
	return der
}

// startCRLServer starts an httptest.Server that serves whatever *crlDER points to.
// Set *crlDER before making any requests.
func startCRLServer(t *testing.T, crlDER *[]byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pkix-crl")
		w.Write(*crlDER)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// resetGlobals restores all package-level state touched by certauth functions.
// Must be called at the top of every test; do not use t.Parallel().
func resetGlobals() {
	revocationCacheMu.Lock()
	revocationCache = map[string]revocationEntry{}
	revocationCacheMu.Unlock()

	crlCacheMu.Lock()
	crlCache = map[string]*x509.RevocationList{}
	crlCacheMu.Unlock()

	issuerCerts = map[string]*x509.Certificate{}
	config = ConfigSettings{}
}

// setupDNAuth populates issuerCerts and config.Main.ClientAuthDNs for pki.clientCert.
func setupDNAuth(pki *testPKI) {
	issuerCerts[string(pki.clientCert.RawIssuer)] = pki.caCert
	config.Main.ClientAuthDNs = []string{subjectToSlashDN(pki.clientCert.Subject)}
}

// writePEMCert writes cert.Raw as a PEM file to path.
func writePEMCert(t *testing.T, path string, cert *x509.Certificate) {
	t.Helper()
	var buf bytes.Buffer
	if err := pem.Encode(&buf, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
}

// ---- unit tests ----

func TestRdnValueToString(t *testing.T) {
	tests := []struct {
		name string
		val  interface{}
		want string
	}{
		{"string passthrough", "hello", "hello"},
		{"byte slice to string", []byte("world"), "world"},
		{"integer falls back to fmt", 42, "42"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := rdnValueToString(tc.val); got != tc.want {
				t.Errorf("rdnValueToString(%v) = %q, want %q", tc.val, got, tc.want)
			}
		})
	}
}

// TestLoadIssuerCACerts_RawBytesKey verifies that issuerCerts is keyed by raw DER
// Subject bytes, so a client cert lookup via cert.RawIssuer always finds the CA cert
// regardless of ASN.1 string-type encoding differences.
func TestLoadIssuerCACerts_RawBytesKey(t *testing.T) {
	resetGlobals()

	pki := newTestPKI(t, "", "")
	caFile := t.TempDir() + "/ca.pem"
	writePEMCert(t, caFile, pki.caCert)

	if err := loadIssuerCACerts([]string{caFile}); err != nil {
		t.Fatalf("loadIssuerCACerts: %v", err)
	}

	if issuerCerts[string(pki.clientCert.RawIssuer)] == nil {
		t.Error("issuerCerts lookup by clientCert.RawIssuer returned nil; raw-bytes keying is broken")
	}
}

// ---- checkCertAuth integration tests ----

func TestCheckCertAuth_AllowedValidCert(t *testing.T) {
	resetGlobals()

	ocspSrv := startOCSPResponder(t, ocsp.Good)
	pki := newTestPKI(t, ocspSrv.URL, "")
	ocspSrv.pki = pki
	setupDNAuth(pki)

	if err := checkCertAuth(pki.clientCert); err != nil {
		t.Errorf("expected nil for a valid, non-revoked cert; got: %v", err)
	}
}

func TestCheckCertAuth_RevokedByOCSP(t *testing.T) {
	resetGlobals()

	ocspSrv := startOCSPResponder(t, ocsp.Revoked)
	pki := newTestPKI(t, ocspSrv.URL, "")
	ocspSrv.pki = pki
	setupDNAuth(pki)

	err := checkCertAuth(pki.clientCert)
	if err == nil {
		t.Fatal("expected error for a cert OCSP reports as revoked; got nil")
	}
	if _, ok := err.(*revokedError); !ok {
		t.Errorf("expected *revokedError, got %T: %v", err, err)
	}
}

func TestCheckCertAuth_OCSPDown_SoftFail(t *testing.T) {
	resetGlobals()

	// Start a server then immediately shut it down so the URL is unreachable.
	dead := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	pki := newTestPKI(t, deadURL, "")
	setupDNAuth(pki)

	// Unreachable OCSP must soft-fail — monitoring must not be blocked by PKI blips.
	if err := checkCertAuth(pki.clientCert); err != nil {
		t.Errorf("expected nil (soft-fail on unreachable OCSP); got: %v", err)
	}
}

// ---- checkOCSP unit tests ----

// TestCheckOCSP_ReplayOtherSerial ensures that a valid CA-signed OCSP "Good" response
// issued for a different cert serial cannot be replayed to authenticate our cert.
// This exercises the ParseResponseForCert serial-binding check.
func TestCheckOCSP_ReplayOtherSerial(t *testing.T) {
	resetGlobals()

	var pki *testPKI // set after server starts

	// Responder always answers "Good" for serial 99, regardless of what was asked.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		template := ocsp.Response{
			Status:       ocsp.Good,
			SerialNumber: big.NewInt(99), // not the cert under check (serial 42)
			ThisUpdate:   time.Now(),
			NextUpdate:   time.Now().Add(time.Hour),
		}
		respBytes, err := ocsp.CreateResponse(pki.caCert, pki.caCert, template, pki.caKey)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/ocsp-response")
		w.Write(respBytes)
	}))
	defer srv.Close()

	pki = newTestPKI(t, srv.URL, "")

	// checkOCSP must return an error: the response is for serial 99, not our cert (serial 42).
	_, _, err := checkOCSP(pki.clientCert, pki.caCert)
	if err == nil {
		t.Error("expected error: OCSP response for serial 99 must not be accepted for cert with serial 42")
	}
}

// ---- checkCRL unit tests ----

func TestCheckCRL_ValidCertNotRevoked(t *testing.T) {
	resetGlobals()

	var crlDER []byte
	crlSrv := startCRLServer(t, &crlDER)
	pki := newTestPKI(t, "", crlSrv.URL)
	crlDER = buildCRL(t, pki, nil /* no revoked serials */, false)

	revoked, _, err := checkCRL(pki.clientCert, pki.caCert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if revoked {
		t.Error("expected revoked=false for a cert not in the CRL")
	}
}

func TestCheckCRL_ValidCertRevoked(t *testing.T) {
	resetGlobals()

	var crlDER []byte
	crlSrv := startCRLServer(t, &crlDER)
	pki := newTestPKI(t, "", crlSrv.URL) // client cert serial = 42
	crlDER = buildCRL(t, pki, []*big.Int{big.NewInt(42)}, false)

	revoked, _, err := checkCRL(pki.clientCert, pki.caCert)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !revoked {
		t.Error("expected revoked=true for a cert whose serial is in the CRL")
	}
}

// TestCheckCRL_TamperedSignature verifies that a CRL with a corrupted signature is
// rejected. Critically, it must not return (false, _, nil) — that result would mean
// the attacker's CRL (with the revoked serial omitted) was trusted as "cert is good".
func TestCheckCRL_TamperedSignature(t *testing.T) {
	resetGlobals()

	var crlDER []byte
	crlSrv := startCRLServer(t, &crlDER)
	pki := newTestPKI(t, "", crlSrv.URL)
	// Build a CRL that omits serial 42 (attacker scrubbed it), then corrupt the sig.
	crlDER = buildCRL(t, pki, nil, true /* tamper */)

	revoked, _, err := checkCRL(pki.clientCert, pki.caCert)
	if err == nil {
		t.Errorf("expected error for tampered CRL; got revoked=%v err=nil — "+
			"tampered CRL must not be accepted as evidence the cert is valid", revoked)
	}
}

// TestCheckCRL_NoIssuerSkipped verifies that a CRL cannot be used when the issuer
// cert is not loaded: without it the signature is unverifiable, so the CRL is skipped.
func TestCheckCRL_NoIssuerSkipped(t *testing.T) {
	resetGlobals()

	var crlDER []byte
	crlSrv := startCRLServer(t, &crlDER)
	pki := newTestPKI(t, "", crlSrv.URL)
	crlDER = buildCRL(t, pki, nil, false)

	_, _, err := checkCRL(pki.clientCert, nil /* issuer not loaded */)
	if err == nil {
		t.Error("expected error when issuer is nil (CRL signature cannot be verified); got nil")
	}
}
