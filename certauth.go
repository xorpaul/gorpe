package main

import (
	"bytes"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log"
	"math/big"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/crypto/ocsp"
)

// oidToAttrName maps ASN.1 OID dotted strings to their X.500 attribute short names.
var oidToAttrName = map[string]string{
	"2.5.4.6":                   "C",
	"2.5.4.10":                  "O",
	"2.5.4.11":                  "OU",
	"2.5.4.3":                   "CN",
	"2.5.4.7":                   "L",
	"2.5.4.8":                   "ST",
	"2.5.4.5":                   "serialNumber",
	"0.9.2342.19200300.100.1.1": "UID",
	"1.2.840.113549.1.9.1":      "emailAddress",
}

// rdnValueToString converts an ASN.1 attribute value to a string for DN formatting.
// Using fmt.Sprintf("%v") on the raw interface{} breaks for []byte values (produces
// decimal slice notation), so we handle the concrete types explicitly.
func rdnValueToString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// subjectToSlashDN formats a pkix.Name as OpenSSL slash notation: /C=.../O=.../CN=...
// It iterates pkix.Name.Names (raw ASN.1 order) so that UID and other non-typed
// attributes are included and the component order matches OpenSSL's output.
func subjectToSlashDN(name pkix.Name) string {
	var result string
	for _, rdn := range name.Names {
		attrName, ok := oidToAttrName[rdn.Type.String()]
		if !ok {
			attrName = rdn.Type.String()
		}
		result += "/" + attrName + "=" + rdnValueToString(rdn.Value)
	}
	return result
}

// issuerCerts maps a CA cert's raw Subject DER bytes (as string key) to its parsed
// certificate. Using the raw DER avoids mismatches caused by ASN.1 string-type
// differences (PrintableString vs UTF8String) that produce the same text but
// different pkix.Name.Names slices. Populated at startup; read-only after that.
var issuerCerts = map[string]*x509.Certificate{}

// loadIssuerCACerts reads PEM files from caFiles and populates issuerCerts.
func loadIssuerCACerts(caFiles []string) error {
	for _, caFile := range caFiles {
		data, err := os.ReadFile(caFile)
		if err != nil {
			return fmt.Errorf("reading issuer CA file %s: %w", caFile, err)
		}
		for len(data) > 0 {
			var block *pem.Block
			block, data = pem.Decode(data)
			if block == nil {
				break
			}
			if block.Type != "CERTIFICATE" {
				continue
			}
			cert, err := x509.ParseCertificate(block.Bytes)
			if err != nil {
				return fmt.Errorf("parsing certificate in %s: %w", caFile, err)
			}
			dn := subjectToSlashDN(cert.Subject)
			issuerCerts[string(cert.RawSubject)] = cert
			log.Printf("loaded issuer CA cert: %s", dn)
		}
	}
	startCachePruner()
	return nil
}

// revocationEntry caches an OCSP/CRL result until the CA's stated NextUpdate.
type revocationEntry struct {
	revoked   bool
	expiresAt time.Time
}

var (
	revocationCache   = map[string]revocationEntry{}
	revocationCacheMu sync.RWMutex
	crlCache          = map[string]*x509.RevocationList{}
	crlCacheMu        sync.RWMutex
	cachePrunerOnce   sync.Once
)

// startCachePruner launches a background goroutine that removes expired revocation
// cache entries once per hour. Called once from loadIssuerCACerts.
func startCachePruner() {
	cachePrunerOnce.Do(func() {
		go func() {
			ticker := time.NewTicker(time.Hour)
			for range ticker.C {
				now := time.Now()
				revocationCacheMu.Lock()
				for k, e := range revocationCache {
					if now.After(e.expiresAt) {
						delete(revocationCache, k)
					}
				}
				revocationCacheMu.Unlock()
			}
		}()
	})
}

// revokedError is returned by checkRevocation when OCSP or CRL definitively
// confirms the certificate is revoked. It is distinct from transient
// infrastructure errors so callers can decide to hard-fail vs. soft-fail.
type revokedError struct{ msg string }

func (e *revokedError) Error() string { return e.msg }

// revocationClient is used for OCSP and CRL HTTP fetches. A finite timeout
// prevents a slow or unreachable PKI server from blocking handler goroutines.
var revocationClient = &http.Client{Timeout: 10 * time.Second}

// checkCertAuth verifies cert against configured client_auth_dns / client_auth_issuer
// and checks revocation status. Returns nil if the certificate is authorised.
func checkCertAuth(cert *x509.Certificate) error {
	subjectDN := subjectToSlashDN(cert.Subject)
	issuerDN := subjectToSlashDN(cert.Issuer)

	dnAllowed := false
	for _, allowed := range config.Main.ClientAuthDNs {
		if subjectDN == allowed {
			dnAllowed = true
			break
		}
	}
	if !dnAllowed {
		return fmt.Errorf("client cert DN %q not in client_auth_dns", subjectDN)
	}

	if len(config.Main.ClientAuthIssuer) > 0 {
		issuerAllowed := false
		for _, allowed := range config.Main.ClientAuthIssuer {
			if issuerDN == allowed {
				issuerAllowed = true
				break
			}
		}
		if !issuerAllowed {
			return fmt.Errorf("client cert issuer %q not in client_auth_issuer", issuerDN)
		}
	}

	if err := checkRevocation(cert, subjectDN); err != nil {
		// A revokedError means OCSP/CRL definitively confirmed revocation — hard-fail.
		// Any other error is a transient infrastructure problem — soft-fail to avoid
		// fleet-wide monitoring blindness on PKI blips.
		if _, isRevoked := err.(*revokedError); isRevoked {
			return err
		}
		log.Printf("WARN: revocation check failed for %s: %v (soft-fail, allowing)", subjectDN, err)
	}

	return nil
}

// checkRevocation checks cert revocation via OCSP (preferred) then CRL, with caching.
// It returns a *revokedError when the cert is definitively revoked, or a plain error
// when the check could not be completed due to infrastructure problems.
func checkRevocation(cert *x509.Certificate, subjectDN string) error {
	issuerDN := subjectToSlashDN(cert.Issuer)
	cacheKey := cert.SerialNumber.String() + "|" + issuerDN

	revocationCacheMu.RLock()
	entry, cached := revocationCache[cacheKey]
	revocationCacheMu.RUnlock()
	if cached && time.Now().Before(entry.expiresAt) {
		if entry.revoked {
			return &revokedError{fmt.Sprintf("certificate %s is revoked (cached)", subjectDN)}
		}
		return nil
	}

	issuer := issuerCerts[string(cert.RawIssuer)]
	if issuer != nil && len(cert.OCSPServer) > 0 {
		revoked, nextUpdate, err := checkOCSP(cert, issuer)
		if err == nil {
			ttl := nextUpdate
			if ttl.IsZero() {
				ttl = time.Now().Add(1 * time.Hour)
			}
			revocationCacheMu.Lock()
			revocationCache[cacheKey] = revocationEntry{revoked: revoked, expiresAt: ttl}
			revocationCacheMu.Unlock()
			if revoked {
				return &revokedError{fmt.Sprintf("certificate %s is revoked (OCSP)", subjectDN)}
			}
			return nil
		}
		log.Printf("OCSP check failed for %s (serial %s): %v, falling back to CRL", subjectDN, cert.SerialNumber, err)
	}

	if len(cert.CRLDistributionPoints) > 0 {
		revoked, nextUpdate, err := checkCRL(cert, issuer)
		if err == nil {
			ttl := nextUpdate
			if ttl.IsZero() {
				ttl = time.Now().Add(1 * time.Hour)
			}
			revocationCacheMu.Lock()
			revocationCache[cacheKey] = revocationEntry{revoked: revoked, expiresAt: ttl}
			revocationCacheMu.Unlock()
			if revoked {
				return &revokedError{fmt.Sprintf("certificate %s is revoked (CRL)", subjectDN)}
			}
			return nil
		}
		return fmt.Errorf("CRL check failed for %s: %w", subjectDN, err)
	}

	return fmt.Errorf("no OCSP server or CRL distribution point in certificate %s", subjectDN)
}

func checkOCSP(cert, issuer *x509.Certificate) (revoked bool, nextUpdate time.Time, err error) {
	req, err := ocsp.CreateRequest(cert, issuer, nil)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("creating OCSP request: %w", err)
	}
	for _, server := range cert.OCSPServer {
		resp, err := revocationClient.Post(server, "application/ocsp-request", bytes.NewReader(req))
		if err != nil {
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			continue
		}
		// Pass issuer so the OCSP response signature is verified.
		parsed, err := ocsp.ParseResponse(body, issuer)
		if err != nil {
			continue
		}
		return parsed.Status == ocsp.Revoked, parsed.NextUpdate, nil
	}
	return false, time.Time{}, fmt.Errorf("no OCSP server responded for %v", cert.OCSPServer)
}

// checkCRL fetches and checks CRL revocation for cert. issuer is used to verify
// the CRL signature; if nil, signature verification is skipped (and a warning logged).
func checkCRL(cert *x509.Certificate, issuer *x509.Certificate) (revoked bool, nextUpdate time.Time, err error) {
	for _, dp := range cert.CRLDistributionPoints {
		crlCacheMu.RLock()
		cached, ok := crlCache[dp]
		crlCacheMu.RUnlock()

		var crl *x509.RevocationList
		if ok && time.Now().Before(cached.NextUpdate) {
			crl = cached
		} else {
			resp, err := revocationClient.Get(dp)
			if err != nil {
				continue
			}
			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				continue
			}
			var derBytes []byte
			if block, _ := pem.Decode(body); block != nil {
				derBytes = block.Bytes
			} else {
				derBytes = body
			}
			crl, err = x509.ParseRevocationList(derBytes)
			if err != nil {
				continue
			}
			if issuer != nil {
				if err := crl.CheckSignatureFrom(issuer); err != nil {
					log.Printf("WARN: CRL from %s has invalid signature: %v", dp, err)
					continue
				}
			} else {
				log.Printf("WARN: cannot verify CRL signature from %s: issuer cert not loaded", dp)
			}
			crlCacheMu.Lock()
			crlCache[dp] = crl
			crlCacheMu.Unlock()
		}

		serial := new(big.Int).Set(cert.SerialNumber)
		for _, rc := range crl.RevokedCertificateEntries {
			if rc.SerialNumber.Cmp(serial) == 0 {
				return true, crl.NextUpdate, nil
			}
		}
		return false, crl.NextUpdate, nil
	}
	return false, time.Time{}, fmt.Errorf("could not fetch any CRL from %v", cert.CRLDistributionPoints)
}
