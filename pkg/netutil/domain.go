package netutil

import "regexp"

// domainRE matches valid DNS domain names per RFC 1123.
// Labels: alphanumeric + hyphens, must not start/end with hyphen, max 63 chars each.
// Total length max 253 chars.
var domainRE = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// IsValidDomain returns true if the given string is a syntactically valid DNS domain name.
func IsValidDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	return domainRE.MatchString(domain)
}
