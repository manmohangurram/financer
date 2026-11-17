package netguard

import (
	"net"
	"strings"
	"testing"
)

func TestIsBlocked(t *testing.T) {
	blocked := []string{"169.254.169.254", "169.254.0.1", "0.0.0.0", "255.255.255.255", "fe80::1", "ff02::1"}
	for _, raw := range blocked {
		if ip := net.ParseIP(raw); ip == nil || !IsBlocked(ip) {
			t.Errorf("%s should be blocked", raw)
		}
	}
	allowed := []string{"127.0.0.1", "10.0.0.5", "192.168.1.10", "8.8.8.8", "::1", "fd00::1"}
	for _, raw := range allowed {
		if ip := net.ParseIP(raw); ip == nil || IsBlocked(ip) {
			t.Errorf("%s should be allowed", raw)
		}
	}
}

func TestHostnameRejectsBlockedLiteral(t *testing.T) {
	for _, raw := range []string{"169.254.169.254", "0.0.0.0", "fe80::1"} {
		if _, err := Hostname(raw); err == nil {
			t.Errorf("%s should be rejected", raw)
		}
	}
	got, err := Hostname("  API.DeepSeek.com ")
	if err != nil || got != "api.deepseek.com" {
		t.Fatalf("hostname = %q err=%v", got, err)
	}
	if _, err := Hostname("http://x"); err == nil || !strings.Contains(err.Error(), "bare hostname") {
		t.Fatalf("URL should be rejected: %v", err)
	}
}
