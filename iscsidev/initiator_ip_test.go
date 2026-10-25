package iscsidev

import (
	"errors"
	"testing"
)

func TestGetInitiatorIP(t *testing.T) {
	lookupErr := errors.New("session query failed or has ambiguous portals")
	selectionErr := errors.New("new attachment selector failed")
	tests := []struct {
		name       string
		sessionIP  string
		sessionErr error
		newIP      string
		newErr     error
		wantIP     string
		wantErr    error
		wantNew    bool
	}{
		{name: "reload retains old IPv4 despite IPv6 policy", sessionIP: "192.0.2.10", newIP: "2001:db8::10", wantIP: "192.0.2.10"},
		{name: "rescan retains old IPv6 despite storage policy", sessionIP: "2001:db8::10", newIP: "198.51.100.10", wantIP: "2001:db8::10"},
		{name: "existing portal ignores invalid new context", sessionIP: "192.0.2.10", newErr: selectionErr, wantIP: "192.0.2.10"},
		{name: "no session uses current selector", newIP: "2001:db8::10", wantIP: "2001:db8::10", wantNew: true},
		{name: "no session propagates selector error", newErr: selectionErr, wantErr: selectionErr, wantNew: true},
		{name: "query error falls back to current selector", sessionErr: lookupErr, newIP: "192.0.2.10", wantIP: "192.0.2.10", wantNew: true},
		{name: "query error fallback propagates selector error", sessionErr: lookupErr, newErr: selectionErr, wantErr: selectionErr, wantNew: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			selected := false
			ip, err := getInitiatorIP("volume-target", func(target string) (string, error) {
				if target != "volume-target" {
					t.Fatalf("queried target %q", target)
				}
				return tt.sessionIP, tt.sessionErr
			}, func() (string, error) {
				selected = true
				return tt.newIP, tt.newErr
			})
			if ip != tt.wantIP || !errors.Is(err, tt.wantErr) {
				t.Fatalf("IP, error = %q, %v; want %q, %v", ip, err, tt.wantIP, tt.wantErr)
			}
			if selected != tt.wantNew {
				t.Fatalf("new attachment selection called = %v, want %v", selected, tt.wantNew)
			}
		})
	}
}
