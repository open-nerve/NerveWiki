package httpadapter

import "github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"

// ReadCause is readCause, for the tests outside the package.
func ReadCause(err error) string { return readCause(err) }

// Signature is signature, for the tests outside the package.
func Signature(s string) bool { return signature(s) }

// UploadPolicy is uploadPolicy, for the tests outside the package.
func UploadPolicy(l Limits) httpserver.StreamPolicy { return uploadPolicy(l) }

// DownloadPolicy is downloadPolicy, for the tests outside the package.
func DownloadPolicy(l Limits) httpserver.StreamPolicy { return downloadPolicy(l) }
