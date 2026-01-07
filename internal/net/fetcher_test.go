package net

import "testing"

func TestIsIPv6(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"IPv4 simple", "192.168.1.1", false},
		{"IPv4 public", "8.8.8.8", false},
		{"IPv6 full", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", true},
		{"IPv6 compressed", "2001:db8::1", true},
		{"IPv6 loopback", "::1", true},
		{"IPv6 link-local", "fe80::1", true},
		{"IPv6 with zone", "fe80::1%eth0", true}, // zone ID contains colon separator
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsIPv6(tt.ip)
			if result != tt.expected {
				t.Errorf("IsIPv6(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}

func TestFormatExternalAddress(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		port     int
		expected string
	}{
		{
			name:     "IPv4 standard port",
			ip:       "192.168.1.1",
			port:     26656,
			expected: "tcp://192.168.1.1:26656",
		},
		{
			name:     "IPv4 custom port",
			ip:       "8.8.8.8",
			port:     12345,
			expected: "tcp://8.8.8.8:12345",
		},
		{
			name:     "IPv6 compressed",
			ip:       "2001:db8::1",
			port:     26656,
			expected: "tcp://[2001:db8::1]:26656",
		},
		{
			name:     "IPv6 full",
			ip:       "2001:0db8:85a3:0000:0000:8a2e:0370:7334",
			port:     26656,
			expected: "tcp://[2001:0db8:85a3:0000:0000:8a2e:0370:7334]:26656",
		},
		{
			name:     "IPv6 loopback",
			ip:       "::1",
			port:     26656,
			expected: "tcp://[::1]:26656",
		},
		{
			name:     "IPv6 real world example",
			ip:       "2a01:4f9:c012:99f3::1",
			port:     26656,
			expected: "tcp://[2a01:4f9:c012:99f3::1]:26656",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatExternalAddress(tt.ip, tt.port)
			if result != tt.expected {
				t.Errorf("FormatExternalAddress(%q, %d) = %q, want %q",
					tt.ip, tt.port, result, tt.expected)
			}
		})
	}
}

func TestIsPublicIP(t *testing.T) {
	tests := []struct {
		name     string
		ip       string
		expected bool
	}{
		{"empty", "", false},
		{"localhost IPv4", "127.0.0.1", false},
		{"localhost IPv6", "::1", false},
		{"localhost string", "localhost", false},
		{"private 10.x", "10.0.0.1", false},
		{"private 172.16.x", "172.16.0.1", false},
		{"private 172.31.x", "172.31.255.255", false},
		{"private 192.168.x", "192.168.1.1", false},
		{"link-local", "169.254.1.1", false},
		{"public IPv4", "8.8.8.8", true},
		{"public IPv4 2", "1.1.1.1", true},
		{"public Hetzner", "135.181.202.153", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsPublicIP(tt.ip)
			if result != tt.expected {
				t.Errorf("IsPublicIP(%q) = %v, want %v", tt.ip, result, tt.expected)
			}
		})
	}
}
