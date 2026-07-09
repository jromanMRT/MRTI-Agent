//go:build !linux

package network

// systemResolvers / systemGateway are best-effort and currently only
// implemented for Linux. On other platforms (Windows, macOS) these return
// empty and can be extended with registry/route-table lookups later.
func systemResolvers() []string { return nil }
func systemGateway() string     { return "" }
