package main

import "testing"

func TestNormalizeNodeAddress(t *testing.T) {
	tests := map[string]string{
		"vpn.example.com": "vpn.example.com",
		"VPN.Example.COM.": "vpn.example.com",
		"198.35.45.175": "198.35.45.175",
		"[2001:db8::1]": "2001:db8::1",
	}
	for in, want := range tests {
		got, err := normalizeNodeAddress(in)
		if err != nil || got != want {
			t.Fatalf("normalizeNodeAddress(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "https://vpn.example.com", "vpn.example.com:443", "bad host", "localhost"} {
		if _, err := normalizeNodeAddress(bad); err == nil {
			t.Fatalf("expected invalid node address: %q", bad)
		}
	}
}

func TestNormalizeDNSName(t *testing.T) {
	if got, err := normalizeDNSName("www.apple.com"); err != nil || got != "www.apple.com" {
		t.Fatalf("unexpected apple DNS result: %q, %v", got, err)
	}
	for _, bad := range []string{"www.apple.com/path", "-apple.com", "apple-.com", "apple", "apple_com"} {
		if _, err := normalizeDNSName(bad); err == nil {
			t.Fatalf("expected invalid DNS name: %q", bad)
		}
	}
}

func TestValidateEditedPortRejectsOtherNode(t *testing.T) {
	nodes := []Node{{ID: "a", Port: 20001}, {ID: "b", Port: 20002}}
	if err := validateEditedPort(20002, "a", nodes); err == nil {
		t.Fatal("expected duplicate node port to be rejected")
	}
}
