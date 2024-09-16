//go:build go1.20
// +build go1.20

package http

import "crypto/tls"

func isCertError(err error) bool {
	_, ok := err.(*tls.CertificateVerificationError)
	return ok
}
