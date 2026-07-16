package application

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"sort"

	"switchyard.dev/switchyard/internal/ports/domain"
)

// Classify returns stable pairwise conflicts with the most specific explanation.
func Classify(facts []domain.Fact) []domain.Conflict {
	var conflicts []domain.Conflict
	for left := 0; left < len(facts); left++ {
		for right := left + 1; right < len(facts); right++ {
			first, second := facts[left], facts[right]
			if first.Port != second.Port || sameClaim(first, second) || !hostsOverlap(first.Host, second.Host) {
				continue
			}
			kind, ok := conflictKind(first, second)
			if !ok {
				continue
			}
			ids := []string{first.ID, second.ID}
			sort.Strings(ids)
			digest := sha256.Sum256([]byte(ids[0] + "\x00" + ids[1] + "\x00" + string(kind)))
			conflicts = append(conflicts, domain.Conflict{
				ID: "portconflict_" + hex.EncodeToString(digest[:8]), Type: kind, Port: first.Port,
				Summary: conflictSummary(kind, first.Port), Facts: []domain.Fact{first, second},
			})
		}
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].ID < conflicts[j].ID })
	return conflicts
}

func conflictKind(first, second domain.Fact) (domain.ConflictType, bool) {
	firstUnknown := first.Kind == domain.KindBinding && first.ProjectID == ""
	secondUnknown := second.Kind == domain.KindBinding && second.ProjectID == ""
	if firstUnknown && secondUnknown {
		return "", false
	}
	if first.Protocol != second.Protocol {
		return domain.ConflictProtocolMismatch, true
	}
	if firstUnknown || secondUnknown {
		return domain.ConflictUnknownBinding, true
	}
	if normalizedHost(first.Host) != normalizedHost(second.Host) {
		return domain.ConflictHostOverlap, true
	}
	pair := string(first.Kind) + ":" + string(second.Kind)
	switch pair {
	case "declaration:declaration":
		return domain.ConflictDeclaredDeclared, true
	case "declaration:reservation", "reservation:declaration":
		return domain.ConflictDeclaredReserved, true
	case "declaration:binding", "binding:declaration", "reservation:binding", "binding:reservation":
		return domain.ConflictDeclaredBound, true
	case "reservation:reservation":
		return domain.ConflictReservedReserved, true
	default:
		return "", false
	}
}

func sameClaim(first, second domain.Fact) bool {
	if first.ProjectID == "" || second.ProjectID == "" {
		return first.ProjectID == "" && second.ProjectID == "" && first.ProcessID > 0 && first.ProcessID == second.ProcessID
	}
	if first.ProjectID != second.ProjectID || first.Protocol != second.Protocol {
		return false
	}
	if first.PortID != "" && first.PortID == second.PortID {
		return true
	}
	oneIsBinding := first.Kind == domain.KindBinding || second.Kind == domain.KindBinding
	return oneIsBinding && first.ServiceID != "" && first.ServiceID == second.ServiceID
}

func hostsOverlap(first, second string) bool {
	left, right := normalizedHost(first), normalizedHost(second)
	return left == right || isWildcard(left) || isWildcard(right)
}

func normalizedHost(host string) string {
	if host == "" || host == "*" {
		return "0.0.0.0"
	}
	if host == "localhost" {
		return "127.0.0.1"
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.String()
	}
	return host
}

func isWildcard(host string) bool { return host == "0.0.0.0" || host == "::" }

func conflictSummary(kind domain.ConflictType, port int) string {
	return fmt.Sprintf("%s on local port %d", kind, port)
}
