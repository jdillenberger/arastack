package mdns

import (
	"regexp"
	"strings"
)

var (
	allowIfacesRe      = regexp.MustCompile(`(?m)^#?\s*allow-interfaces\s*=.*$`)
	allowP2PRe         = regexp.MustCompile(`(?m)^#?\s*allow-point-to-point\s*=.*$`)
	enableReflectorRe  = regexp.MustCompile(`(?m)^#?\s*enable-reflector\s*=.*$`)
)

// BuildAvahiConfig applies the desired interface list and reflector setting to
// an avahi-daemon.conf content string. It is a pure function.
func BuildAvahiConfig(content string, ifaces []string, enableReflector bool) string {
	ifaceList := strings.Join(ifaces, ",")
	directive := "allow-interfaces=" + ifaceList

	// Update or insert allow-interfaces directive.
	if allowIfacesRe.MatchString(content) {
		content = allowIfacesRe.ReplaceAllString(content, directive)
	} else if strings.Contains(content, "[server]") {
		content = strings.Replace(content, "[server]\n", "[server]\n"+directive+"\n", 1)
	}

	// Enable point-to-point support when reflector is active (VPN interfaces like
	// wg0 are POINTOPOINT and Avahi ignores them without this setting).
	p2pDirective := "allow-point-to-point=yes"
	if !enableReflector {
		p2pDirective = "#allow-point-to-point=no"
	}
	if allowP2PRe.MatchString(content) {
		content = allowP2PRe.ReplaceAllString(content, p2pDirective)
	} else if strings.Contains(content, "[server]") {
		content = strings.Replace(content, directive+"\n", directive+"\n"+p2pDirective+"\n", 1)
	}

	// Handle reflector section.
	if enableReflector {
		reflectorBlock := "[reflector]\nenable-reflector=yes\n"
		if strings.Contains(content, "[reflector]") {
			// Update existing reflector section — set enable-reflector=yes.
			if enableReflectorRe.MatchString(content) {
				content = enableReflectorRe.ReplaceAllString(content, "enable-reflector=yes")
			} else {
				content = strings.Replace(content, "[reflector]\n", "[reflector]\nenable-reflector=yes\n", 1)
			}
		} else {
			// Append reflector section.
			if !strings.HasSuffix(content, "\n") {
				content += "\n"
			}
			content += "\n" + reflectorBlock
		}
	} else if enableReflectorRe.MatchString(content) {
		// If reflector should be off, disable it if present.
		content = enableReflectorRe.ReplaceAllString(content, "#enable-reflector=no")
	}

	return content
}
