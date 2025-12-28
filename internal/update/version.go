// Package update provides self-update functionality for mono-commander.
package update

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Version represents a semantic version.
type Version struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease string
	Raw        string
}

// ParseVersion parses a version string into a Version struct.
// Accepts formats like: v1.2.3, 1.2.3, v1.2.3-beta.1, etc.
func ParseVersion(s string) (Version, error) {
	raw := s
	s = strings.TrimPrefix(s, "v")

	if s == "" || s == "dev" {
		return Version{Raw: raw}, nil
	}

	// Split prerelease
	var prerelease string
	if idx := strings.IndexAny(s, "-+"); idx != -1 {
		prerelease = s[idx+1:]
		s = s[:idx]
	}

	parts := strings.Split(s, ".")
	if len(parts) < 1 {
		return Version{}, fmt.Errorf("invalid version format: %s", raw)
	}

	var major, minor, patch int
	var err error

	major, err = strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	if len(parts) > 1 {
		minor, err = strconv.Atoi(parts[1])
		if err != nil {
			return Version{}, fmt.Errorf("invalid minor version: %s", parts[1])
		}
	}

	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return Version{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Raw:        raw,
	}, nil
}

// String returns the version as a string.
func (v Version) String() string {
	if v.Raw != "" {
		return v.Raw
	}
	s := fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		s += "-" + v.Prerelease
	}
	return s
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	// Prerelease comparison: no prerelease > prerelease
	if v.Prerelease == "" && other.Prerelease != "" {
		return 1
	}
	if v.Prerelease != "" && other.Prerelease == "" {
		return -1
	}
	if v.Prerelease != other.Prerelease {
		if v.Prerelease < other.Prerelease {
			return -1
		}
		return 1
	}

	return 0
}

// LessThan returns true if v < other.
func (v Version) LessThan(other Version) bool {
	return v.Compare(other) < 0
}

// IsZero returns true if this is an empty/zero version.
func (v Version) IsZero() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0 && v.Prerelease == ""
}

// IsDev returns true if this is a development version.
func (v Version) IsDev() bool {
	return v.Raw == "dev" || v.Raw == ""
}

// AssetNamePattern returns a regex pattern for matching release assets.
func AssetNamePattern(os, arch string) *regexp.Regexp {
	// Normalize arch names
	goArch := arch
	if arch == "x86_64" {
		goArch = "amd64"
	}
	if arch == "aarch64" {
		goArch = "arm64"
	}

	// Match patterns like: monoctl_darwin_amd64, monoctl-linux-arm64.tar.gz, etc.
	pattern := fmt.Sprintf(`(?i)monoctl[_-]%s[_-]%s`, os, goArch)
	return regexp.MustCompile(pattern)
}
