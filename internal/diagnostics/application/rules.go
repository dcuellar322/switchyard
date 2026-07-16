package application

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"switchyard.dev/switchyard/internal/diagnostics/domain"
)

type logRule struct {
	code, title, summary, severity string
	pattern                        *regexp.Regexp
	notifies                       bool
}

var logRules = []logRule{
	{"PORT_BIND_FAILED", "A service could not bind its port", "A recent log reports an address already in use.", "error", regexp.MustCompile(`(?i)(address already in use|eaddrinuse|bind.+failed)`), true},
	{"DEPENDENCY_UNREACHABLE", "A dependency is unreachable", "A recent log reports a refused or failed dependency connection.", "warning", regexp.MustCompile(`(?i)(connection refused|econnrefused|could not connect|dial tcp.+refused)`), true},
	{"PERMISSION_DENIED", "A service lacks required access", "A recent log reports a permission or access-denied failure.", "error", regexp.MustCompile(`(?i)(permission denied|eacces|access is denied)`), false},
	{"RESOURCE_EXHAUSTED", "A host resource was exhausted", "A recent log reports disk or memory exhaustion.", "error", regexp.MustCompile(`(?i)(no space left|out of memory|oomkilled|cannot allocate memory)`), true},
	{"CONFIGURATION_MISSING", "Required configuration is missing", "A recent log reports a missing environment or configuration value.", "warning", regexp.MustCompile(`(?i)(missing (required )?(environment|env|config)|undefined variable|not set)`), false},
}

// Evaluate applies stable known-failure rules without any provider dependency.
func Evaluate(bundle domain.Bundle) []domain.Hypothesis {
	result := []domain.Hypothesis{}
	if bundle.Snapshot.Runtime.Driver != "" && !bundle.Snapshot.Runtime.EngineConnected {
		result = append(result, hypothesis("RUNTIME_DISCONNECTED", "Runtime observer is disconnected", "Switchyard cannot reach the configured runtime, so service state may be incomplete.", "error", 1, []string{"runtime"}, bundle.Actions, true))
	}
	if bundle.ProjectState == "failed" || bundle.ProjectState == "degraded" || bundle.ProjectState == "partially_running" {
		result = append(result, hypothesis("RUNTIME_DEGRADED", "Project runtime is degraded", "One or more declared services are failed, unhealthy, or not running.", "error", .98, []string{"runtime"}, bundle.Actions, true))
	}
	for _, conflict := range bundle.Snapshot.PortConflicts {
		result = append(result, hypothesis("PORT_CONFLICT", "Port conflict detected", conflict.Summary, "error", 1, []string{"port:" + conflict.ID}, bundle.Actions, true))
	}
	for _, failure := range bundle.Snapshot.Health.Failures {
		if !failure.Required && failure.Severity != "error" {
			continue
		}
		result = append(result, hypothesis("UNHEALTHY_DEPENDENCY", "Required health check is failing", fmt.Sprintf("%s: %s", failure.ServiceID, failure.Message), "error", 1, []string{"health:" + failure.ID}, bundle.Actions, true))
	}
	if evidence := repeatedCrashEvidence(bundle); len(evidence) > 0 {
		result = append(result, hypothesis("REPEATED_CRASH", "A service is repeatedly crashing", "Restart counts or recent failed lifecycle runs crossed the repeated-crash threshold.", "error", .99, evidence, bundle.Actions, true))
	}
	if len(bundle.Snapshot.Resources.Warnings) > 0 {
		result = append(result, hypothesis("RESOURCE_PRESSURE", "Sustained resource pressure detected", strings.Join(bundle.Snapshot.Resources.Warnings, " "), "warning", .97, []string{"resources"}, bundle.Actions, true))
	}
	result = append(result, evaluateLogRules(bundle)...)
	if bundle.Snapshot.Git.Conflicted > 0 || bundle.Snapshot.Git.Operation != "" {
		result = append(result, hypothesis("GIT_OPERATION_INCOMPLETE", "Git needs attention", "The repository has conflicts or an incomplete Git operation.", "warning", 1, []string{"git"}, bundle.Actions, false))
	}
	if bundle.ProjectAgeDay >= 30 && bundle.ProjectState == "stopped" {
		result = append(result, hypothesis("STALE_PROJECT", "Project may be stale", fmt.Sprintf("The project has not changed in %d days and its runtime is stopped. This is a recommendation only.", bundle.ProjectAgeDay), "info", .85, []string{"project", "runtime"}, bundle.Actions, false))
	}
	if bundle.Snapshot.Cleanup.Candidates > 0 {
		result = append(result, domain.Hypothesis{
			ID: "det_cleanup_preview", Code: "CLEANUP_PREVIEW", Title: "Reclaimable runtime storage is available",
			Summary:  fmt.Sprintf("Dry-run preview found %d candidates and approximately %d bytes. It cannot execute deletion.", bundle.Snapshot.Cleanup.Candidates, bundle.Snapshot.Cleanup.EstimatedBytes),
			Severity: "info", Confidence: 1, Source: "deterministic", EvidenceIDs: []string{"cleanup"}, SuggestedActions: []domain.SuggestedAction{},
		})
	}
	return deduplicate(result)
}

func evaluateLogRules(bundle domain.Bundle) []domain.Hypothesis {
	result := []domain.Hypothesis{}
	for _, rule := range logRules {
		evidence := []string{}
		for _, line := range bundle.Snapshot.RecentLogs {
			if rule.pattern.MatchString(line.Message) {
				evidence = append(evidence, line.EvidenceID)
			}
		}
		if len(evidence) == 0 {
			continue
		}
		if len(evidence) > 5 {
			evidence = evidence[:5]
		}
		result = append(result, hypothesis(rule.code, rule.title, rule.summary, rule.severity, .96, evidence, bundle.Actions, rule.notifies))
	}
	return result
}

func repeatedCrashEvidence(bundle domain.Bundle) []string {
	evidence := []string{}
	if bundle.Snapshot.FailedRuns >= 3 {
		evidence = append(evidence, "operations")
	}
	if bundle.Snapshot.Resources.RestartCount >= 3 {
		evidence = append(evidence, "resources")
	}
	for _, service := range bundle.Snapshot.Runtime.Services {
		if service.RestartCount >= 3 {
			evidence = append(evidence, "runtime")
			break
		}
	}
	return evidence
}

func hypothesis(code, title, summary, severity string, confidence float64, evidence []string, actions []domain.Action, notifies bool) domain.Hypothesis {
	return domain.Hypothesis{
		ID: "det_" + strings.ToLower(code), Code: code, Title: title, Summary: summary,
		Severity: severity, Confidence: confidence, Source: "deterministic", EvidenceIDs: unique(evidence),
		SuggestedActions: relevantActions(code, actions), Notifies: notifies,
	}
}

func relevantActions(code string, actions []domain.Action) []domain.SuggestedAction {
	terms := map[string][]string{
		"RUNTIME_DEGRADED": {"restart", "health", "doctor"}, "REPEATED_CRASH": {"restart", "logs", "doctor"},
		"PORT_CONFLICT": {"port", "config"}, "PORT_BIND_FAILED": {"port", "config"},
		"UNHEALTHY_DEPENDENCY": {"health", "doctor", "test"}, "DEPENDENCY_UNREACHABLE": {"health", "doctor", "test"},
		"PERMISSION_DENIED": {"doctor", "check"}, "CONFIGURATION_MISSING": {"config", "check"},
		"RESOURCE_PRESSURE": {"doctor", "check"}, "RESOURCE_EXHAUSTED": {"doctor", "check"},
	}
	result := []domain.SuggestedAction{}
	for _, action := range actions {
		if action.Risk == "destructive" || action.Risk == "networked" || action.Risk == "interactive" {
			continue
		}
		haystack := strings.ToLower(action.ID + " " + action.Name + " " + action.Type)
		matched := false
		for _, term := range terms[code] {
			if strings.Contains(haystack, term) {
				matched = true
				break
			}
		}
		if matched {
			result = append(result, domain.SuggestedAction{ActionID: action.ID, Name: action.Name, Risk: action.Risk, Reason: "Existing approved project action relevant to this finding."})
		}
		if len(result) == 3 {
			break
		}
	}
	return result
}

func deduplicate(values []domain.Hypothesis) []domain.Hypothesis {
	seen := map[string]bool{}
	result := []domain.Hypothesis{}
	for _, value := range values {
		key := value.Code + "\x00" + strings.Join(value.EvidenceIDs, ",")
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func unique(values []string) []string {
	seen := map[string]bool{}
	result := []string{}
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
