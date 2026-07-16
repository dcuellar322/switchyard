// Package domain owns local HTTP route validation and resolution states.
package domain

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var hostnameLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// Status is the explicit resolution state of one friendly local hostname.
type Status string

const (
	// StatusDisabled means no local routing listener is configured.
	StatusDisabled Status = "disabled"
	// StatusActive identifies one unambiguous active loopback target.
	StatusActive Status = "active"
	// StatusUnavailable has no currently usable target.
	StatusUnavailable Status = "unavailable"
	// StatusConflict has multiple active candidates for one hostname.
	StatusConflict Status = "conflict"
)

// Candidate binds an environment to an optional active loopback HTTP target.
type Candidate struct {
	ProjectID         string `json:"projectId"`
	EnvironmentID     string `json:"environmentId"`
	Hostname          string `json:"hostname"`
	Target            string `json:"target,omitempty"`
	Active            bool   `json:"active"`
	Available         bool   `json:"available"`
	UnavailableReason string `json:"unavailableReason,omitempty"`
}

// Route is the current safe resolution for one .localhost hostname.
type Route struct {
	Hostname                string    `json:"hostname"`
	Status                  Status    `json:"status"`
	ProjectID               string    `json:"projectId,omitempty"`
	EnvironmentID           string    `json:"environmentId,omitempty"`
	Target                  string    `json:"target,omitempty"`
	Reason                  string    `json:"reason,omitempty"`
	CandidateEnvironmentIDs []string  `json:"candidateEnvironmentIds"`
	UpdatedAt               time.Time `json:"updatedAt"`
}

// NormalizeHostname validates and canonicalizes one subdomain of .localhost.
func NormalizeHostname(value string) (string, error) {
	host := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if host == "" || len(host) > 253 || !strings.HasSuffix(host, ".localhost") {
		return "", errors.New("hostname must be a .localhost subdomain")
	}
	prefix := strings.TrimSuffix(host, ".localhost")
	if prefix == "" {
		return "", errors.New("hostname must include a label before .localhost")
	}
	for _, label := range strings.Split(prefix, ".") {
		if !hostnameLabel.MatchString(label) {
			return "", fmt.Errorf("invalid .localhost hostname label %q", label)
		}
	}
	return host, nil
}

// HostnameFromRequest extracts and validates a route hostname from an HTTP
// Host field, which may include the proxy listener's port.
func HostnameFromRequest(value string) (string, error) {
	value = strings.TrimSpace(value)
	if host, port, err := net.SplitHostPort(value); err == nil {
		number, parseErr := strconv.Atoi(port)
		if parseErr != nil || number < 1 || number > 65535 {
			return "", errors.New("invalid route host port")
		}
		value = host
	} else if strings.Contains(value, ":") {
		return "", errors.New("invalid route host")
	}
	return NormalizeHostname(value)
}

// ValidateTarget permits only plain HTTP loopback upstreams. HTTPS, user info,
// remote hosts, fragments, and embedded configuration queries are rejected.
func ValidateTarget(value string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(value))
	if err != nil {
		return nil, fmt.Errorf("parse local route target: %w", err)
	}
	if target.Scheme != "http" || target.Host == "" {
		return nil, errors.New("route target must be an absolute HTTP URL")
	}
	if target.User != nil || target.RawQuery != "" || target.Fragment != "" {
		return nil, errors.New("route target cannot contain credentials, a query, or a fragment")
	}
	hostname := strings.TrimSuffix(strings.ToLower(target.Hostname()), ".")
	address := net.ParseIP(hostname)
	if hostname != "localhost" && (address == nil || !address.IsLoopback()) {
		return nil, errors.New("route target must use a loopback address or localhost")
	}
	return target, nil
}

// CandidateIDs returns sorted unique environment identities for status output.
func CandidateIDs(candidates []Candidate) []string {
	seen := make(map[string]struct{}, len(candidates))
	result := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.EnvironmentID == "" {
			continue
		}
		if _, exists := seen[candidate.EnvironmentID]; !exists {
			seen[candidate.EnvironmentID] = struct{}{}
			result = append(result, candidate.EnvironmentID)
		}
	}
	sort.Strings(result)
	return result
}
