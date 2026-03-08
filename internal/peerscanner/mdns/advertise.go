package mdns

import (
	"fmt"
	"log/slog"
	"strconv"

	"github.com/grandcat/zeroconf"
)

const ServiceType = "_peer-scanner._tcp"

// Advertise registers the peer-scanner service via mDNS and returns a shutdown function.
func Advertise(hostname string, port int, version string, role string, tags map[string]string) (shutdown func(), err error) {
	txt := []string{
		"hostname=" + hostname,
		"version=" + version,
		"role=" + role,
		"port=" + strconv.Itoa(port),
	}
	for k, v := range tags {
		txt = append(txt, "tag."+k+"="+v)
	}

	ifaces, err := PhysicalInterfaces()
	if err != nil {
		return nil, fmt.Errorf("detecting network interfaces: %w", err)
	}

	server, err := zeroconf.Register(
		hostname,
		ServiceType,
		"local.",
		port,
		txt,
		ifaces,
	)
	if err != nil {
		return nil, fmt.Errorf("registering mDNS service: %w", err)
	}

	slog.Debug("mDNS advertising started", "hostname", hostname, "port", port)

	return func() {
		server.Shutdown()
		slog.Debug("mDNS advertising stopped")
	}, nil
}
