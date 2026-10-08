package httpadapter

// ReadCause is readCause, for the tests outside the package.
func ReadCause(err error) string { return readCause(err) }

// Signature is signature, for the tests outside the package.
func Signature(s string) bool { return signature(s) }
