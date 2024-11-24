package httprepeater

import "crypto/tls"

func isCertError(err error) bool {
	_, ok := err.(*tls.CertificateVerificationError)
	return ok
}
