package util

import (
	"errors"
	"net"
	"strings"
	"testing"

	lhnet "github.com/longhorn/go-common-libs/net"
)

func TestGetIPToHostSelection(t *testing.T) {
	defaultErr := errors.New("default discovery failed")
	tests := []struct {
		name        string
		podIP       string
		defaultErr  error
		wantIP      string
		wantErr     error
		invalid     bool
		wantDefault bool
	}{
		{name: "IPv4 primary", podIP: "192.0.2.10", wantIP: "192.0.2.10"},
		{name: "IPv6 primary", podIP: "2001:db8::10", wantIP: "2001:db8::10"},
		{name: "canonicalize expanded IPv6", podIP: "2001:0db8:0:0:0:0:0:0010", wantIP: "2001:db8::10"},
		{name: "canonicalize IPv6 hex case", podIP: "2001:DB8::A", wantIP: "2001:db8::a"},
		{name: "POD_IP skips the default lookup", podIP: "192.0.2.10", defaultErr: defaultErr, wantIP: "192.0.2.10"},
		{name: "invalid primary", podIP: "not-an-IP", invalid: true},
		{name: "unspecified primary", podIP: "::", invalid: true},
		{name: "loopback primary", podIP: "127.0.0.1", invalid: true},
		{name: "link local primary", podIP: "fe80::1", invalid: true},
		{name: "multicast primary", podIP: "ff02::1", invalid: true},
		{name: "empty primary uses default", wantIP: "203.0.113.10", wantDefault: true},
		{name: "default error preserved", defaultErr: defaultErr, wantErr: defaultErr, wantDefault: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var called bool
			ip, err := getIPToHost(tt.podIP, func() (string, error) {
				called = true
				return getIPFromAddrs([]net.Addr{&net.IPNet{IP: net.ParseIP("203.0.113.10")}}), tt.defaultErr
			})
			if tt.invalid {
				if ip != "" || err == nil || !strings.Contains(err.Error(), "invalid POD_IP") {
					t.Fatalf("IP, error = %q, %v; want invalid POD_IP error", ip, err)
				}
			} else if !errors.Is(err, tt.wantErr) || (err == nil && ip != tt.wantIP) {
				t.Fatalf("IP, error = %q, %v; want %q, %v", ip, err, tt.wantIP, tt.wantErr)
			}
			if called != tt.wantDefault {
				t.Fatalf("default lookup called = %v, want %v", called, tt.wantDefault)
			}
		})
	}
}

func TestGetIPToHostPodEnvironment(t *testing.T) {
	t.Setenv(lhnet.EnvPodIP, "2001:db8::10")
	if ip, err := GetIPToHost(); err != nil || ip != "2001:db8::10" {
		t.Fatalf("IP, error = %q, %v; want POD_IP %q", ip, err, "2001:db8::10")
	}
}

func TestLegacyIPFromAddrs(t *testing.T) {
	addrs := []net.Addr{
		&net.IPNet{IP: net.ParseIP("127.0.0.1")},
		&net.IPNet{IP: net.ParseIP("fe80::1")},
		&net.IPNet{IP: net.ParseIP("2001:db8::1")},
		&net.IPNet{IP: net.ParseIP("192.0.2.1")},
	}
	if got := getIPFromAddrs(addrs); got != "2001:db8::1" {
		t.Fatalf("legacy IP = %q, want first global-unicast address", got)
	}
	if got := getIPFromAddrs(addrs[:2]); got != "" {
		t.Fatalf("legacy IP = %q, want empty for non-global addresses", got)
	}
}
