package update

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected Version
		wantErr  bool
	}{
		{"v1.2.3", Version{Major: 1, Minor: 2, Patch: 3, Raw: "v1.2.3"}, false},
		{"1.2.3", Version{Major: 1, Minor: 2, Patch: 3, Raw: "1.2.3"}, false},
		{"v1.0.0", Version{Major: 1, Minor: 0, Patch: 0, Raw: "v1.0.0"}, false},
		{"v2.1", Version{Major: 2, Minor: 1, Patch: 0, Raw: "v2.1"}, false},
		{"v1.2.3-beta.1", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "beta.1", Raw: "v1.2.3-beta.1"}, false},
		{"v1.2.3-rc1", Version{Major: 1, Minor: 2, Patch: 3, Prerelease: "rc1", Raw: "v1.2.3-rc1"}, false},
		{"dev", Version{Raw: "dev"}, false},
		{"", Version{Raw: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVersion(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseVersion(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got.Major != tt.expected.Major || got.Minor != tt.expected.Minor || got.Patch != tt.expected.Patch {
				t.Errorf("ParseVersion(%q) = %+v, want %+v", tt.input, got, tt.expected)
			}
			if got.Prerelease != tt.expected.Prerelease {
				t.Errorf("ParseVersion(%q).Prerelease = %q, want %q", tt.input, got.Prerelease, tt.expected.Prerelease)
			}
		})
	}
}

func TestVersionCompare(t *testing.T) {
	tests := []struct {
		a, b     string
		expected int
	}{
		{"v1.0.0", "v1.0.0", 0},
		{"v1.0.0", "v1.0.1", -1},
		{"v1.0.1", "v1.0.0", 1},
		{"v1.0.0", "v1.1.0", -1},
		{"v1.1.0", "v1.0.0", 1},
		{"v1.0.0", "v2.0.0", -1},
		{"v2.0.0", "v1.0.0", 1},
		{"v1.0.0-alpha", "v1.0.0", -1}, // prerelease < release
		{"v1.0.0", "v1.0.0-alpha", 1},
		{"v1.0.0-alpha", "v1.0.0-beta", -1},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_vs_"+tt.b, func(t *testing.T) {
			a, _ := ParseVersion(tt.a)
			b, _ := ParseVersion(tt.b)
			got := a.Compare(b)
			if got != tt.expected {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestVersionLessThan(t *testing.T) {
	tests := []struct {
		a, b     string
		expected bool
	}{
		{"v1.0.0", "v1.0.1", true},
		{"v1.0.1", "v1.0.0", false},
		{"v1.0.0", "v1.0.0", false},
		{"v0.9.9", "v1.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.a+"_lt_"+tt.b, func(t *testing.T) {
			a, _ := ParseVersion(tt.a)
			b, _ := ParseVersion(tt.b)
			got := a.LessThan(b)
			if got != tt.expected {
				t.Errorf("LessThan(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.expected)
			}
		})
	}
}

func TestVersionIsDev(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"dev", true},
		{"", true},
		{"v1.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			v, _ := ParseVersion(tt.input)
			got := v.IsDev()
			if got != tt.expected {
				t.Errorf("IsDev(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAssetNamePattern(t *testing.T) {
	tests := []struct {
		os, arch string
		input    string
		match    bool
	}{
		{"darwin", "amd64", "monoctl_darwin_amd64", true},
		{"darwin", "arm64", "monoctl_darwin_arm64", true},
		{"linux", "amd64", "monoctl_linux_amd64", true},
		{"linux", "arm64", "monoctl-linux-arm64", true},
		{"linux", "amd64", "monoctl_linux_arm64", false},
		{"darwin", "amd64", "monoctl_linux_amd64", false},
		{"linux", "x86_64", "monoctl_linux_amd64", true},  // x86_64 -> amd64
		{"linux", "aarch64", "monoctl_linux_arm64", true}, // aarch64 -> arm64
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			pattern := AssetNamePattern(tt.os, tt.arch)
			got := pattern.MatchString(tt.input)
			if got != tt.match {
				t.Errorf("AssetNamePattern(%q, %q).Match(%q) = %v, want %v",
					tt.os, tt.arch, tt.input, got, tt.match)
			}
		})
	}
}
