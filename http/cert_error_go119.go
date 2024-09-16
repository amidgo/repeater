//go:build !go1.20
// +build !go1.20

package http

import "crypto/x509"

func isCertError(err error) bool {
	_, ok := err.(x509.UnknownAuthorityError)
	return ok
}
