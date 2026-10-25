package iscsi

import (
	"bufio"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// Sanitized excerpts of the detailed session output captured in
// findScsiDevice's example, plus its bracketed IPv6 counterpart. Current
// portals deliberately differ: reload must retain the PERSISTENT endpoint.
const ipv4SessionSample = `iSCSI Transport Class version 2.0-870
version 2.1.9
Target: iqn.2019-10.io.longhorn:test-volume (non-flash)
	Current Portal: 198.51.100.10:3260,1
	Persistent Portal: 192.0.2.10:3260,1
		**********
		Interface:
		**********
		Iface Name: default
		SID: 463
		iSCSI Connection State: LOGGED IN
		Attached SCSI devices:
		Host Number: 12 State: running
		scsi12 Channel 00 Id 0 Lun: 1
			Attached scsi disk sdb State: running
`

const ipv6SessionSample = `Target: iqn.2019-10.io.longhorn:test-volume
	Current Portal: [2001:db8:1::10]:3260,1
	Persistent Portal: [2001:db8::10]:3260,1
		SID: 464
		iSCSI Connection State: LOGGED IN
`

func TestParseTargetSessionIP(t *testing.T) {
	const target = "iqn.2019-10.io.longhorn:test-volume"
	tests := []struct {
		name    string
		output  string
		wantIP  string
		wantErr string
	}{
		{name: "IPv4 persistent not current", output: ipv4SessionSample, wantIP: "192.0.2.10"},
		{name: "bracketed IPv6", output: ipv6SessionSample, wantIP: "2001:db8::10"},
		{name: "no sessions"},
		{name: "different target", output: strings.ReplaceAll(ipv4SessionSample, target, target+"-other")},
		{name: "exact target among others", output: strings.ReplaceAll(ipv4SessionSample, target, target+"-other") + ipv6SessionSample, wantIP: "2001:db8::10"},
		{name: "duplicate same portal", output: ipv4SessionSample + ipv4SessionSample, wantIP: "192.0.2.10"},
		{name: "multiple IPv4 portals", output: ipv4SessionSample + strings.ReplaceAll(ipv4SessionSample, "192.0.2.10", "192.0.2.11"), wantErr: "has sessions on distinct portals"},
		{name: "multiple mixed family portals", output: ipv4SessionSample + ipv6SessionSample, wantErr: "has sessions on distinct portals"},
		{name: "multiple portals reversed", output: ipv6SessionSample + ipv4SessionSample, wantErr: "has sessions on distinct portals"},
		{name: "same endpoint IPv6 spelling", output: ipv6SessionSample + strings.ReplaceAll(ipv6SessionSample, "2001:db8::10", "2001:0db8:0:0:0:0:0:10"), wantIP: "2001:db8::10"},
		{name: "persistent spelling returned as printed", output: strings.ReplaceAll(ipv6SessionSample, "[2001:db8::10]", "[2001:0db8:0:0:0:0:0:10]"), wantIP: "2001:0db8:0:0:0:0:0:10"},
		{name: "invalid IP", output: strings.ReplaceAll(ipv4SessionSample, "192.0.2.10", "invalid"), wantErr: "invalid persistent portal"},
		{name: "malformed endpoint", output: strings.ReplaceAll(ipv6SessionSample, "[2001:db8::10]:3260", "2001:db8::10"), wantErr: "invalid persistent portal"},
		{name: "non-default port cannot be silently lost", output: strings.ReplaceAll(ipv4SessionSample, ":3260", ":3261"), wantErr: "unsupported persistent portal port"},
		{name: "missing persistent portal", output: "Target: " + target + "\nCurrent Portal: 192.0.2.10:3260,1\n", wantErr: "no persistent portal"},
		{name: "missing portal before next target", output: "Target: " + target + "\nTarget: other\n", wantErr: "no persistent portal"},
		{name: "unparsable output", output: strings.Repeat("x", 1024)},
		{name: "line exceeds scanner buffer", output: strings.Repeat("x", bufio.MaxScanTokenSize+1), wantErr: "failed to parse"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ip, err := parseTargetSessionIP(tt.output, target)
			if ip != tt.wantIP {
				t.Fatalf("portal IP = %q, want %q", ip, tt.wantIP)
			}
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

type sessionExitError int

func (e sessionExitError) Error() string { return fmt.Sprintf("exit status %d", e) }
func (e sessionExitError) ExitCode() int { return int(e) }

func TestIsNoSessionError(t *testing.T) {
	for _, code := range []sessionExitError{21, 1, 20, 210} {
		t.Run(code.Error(), func(t *testing.T) {
			if got := isNoSessionError(fmt.Errorf("session query: %w", code)); got != (code == 21) {
				t.Fatalf("isNoSessionError = %v for exit code %d", got, int(code))
			}
		})
	}
	if isNoSessionError(errors.New("untyped failure mentioning exit status 21")) {
		t.Fatal("untyped error treated as no session")
	}
}
