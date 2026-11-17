// Package netguard validates hostnames the server will connect to on a user's
// behalf (IMAP servers, AI endpoints). Private and loopback addresses are
// allowed — a LAN mailbox or a local LiteLLM is a valid setup — but link-local,
// metadata, unspecified and multicast targets are not.
package netguard

import (
	"fmt"
	"net"
	"strings"

	"github.com/mohan9182/financer/internal/errors"
)

// Hostname normalises a bare hostname and rejects a blocked literal IP.
// DNS is not resolved here: a Docker service name may only resolve inside the
// running container, and saving a setting must not depend on that.
func Hostname(raw string) (string, error) {
	host := strings.ToLower(strings.TrimSpace(raw))
	if host == "" {
		return "", errors.BadRequest("host is required")
	}
	if strings.ContainsAny(host, " \t\r\n/@") {
		return "", errors.BadRequest("host must be a bare hostname")
	}
	if IsBlockedHost(host) {
		return "", errors.BadRequest("host is a link-local or metadata address")
	}
	return host, nil
}

// IsBlockedHost reports whether the host is a blocked literal address.
func IsBlockedHost(host string) bool {
	ip := net.ParseIP(host)
	return ip != nil && IsBlocked(ip)
}

// Resolve checks every address a hostname currently resolves to. Call it
// immediately before dialling, so a DNS answer pointing at link-local space is
// refused at request time.
func Resolve(host string) error {
	if ip := net.ParseIP(host); ip != nil {
		if IsBlocked(ip) {
			return errors.BadRequest("host is a link-local or metadata address")
		}
		return nil
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return errors.BadGateway(fmt.Sprintf("could not resolve %s", host))
	}
	for _, ip := range ips {
		if IsBlocked(ip) {
			return errors.BadRequest("host resolves to a link-local or metadata address")
		}
	}
	return nil
}

// IsBlocked reports link-local, metadata, unspecified, multicast and broadcast
// addresses.
func IsBlocked(ip net.IP) bool {
	if ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return true
	}
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 255 && v4[1] == 255 && v4[2] == 255 && v4[3] == 255
	}
	return false
}
