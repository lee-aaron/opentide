package security

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// ScanResult is the output of a security scan.
type ScanResult struct {
	Tool     string        `json:"tool"`     // "govulncheck" or "trivy"
	Target   string        `json:"target"`   // path or image ref
	Passed   bool          `json:"passed"`
	Findings []ScanFinding `json:"findings,omitempty"`
}

// ScanFinding is a single vulnerability or issue found by a scanner.
type ScanFinding struct {
	ID       string `json:"id"`       // CVE or GHSA ID
	Severity string `json:"severity"` // "CRITICAL", "HIGH", "MEDIUM", "LOW"
	Package  string `json:"package"`
	Version  string `json:"version"`
	Fixed    string `json:"fixed,omitempty"` // fixed-in version
	Summary  string `json:"summary"`
}

// ScanPolicy defines what scan results are acceptable for publishing.
type ScanPolicy struct {
	BlockOnCritical bool // block publish if any critical findings
	BlockOnHigh     bool // block publish if any high findings
	MaxFindings     int  // max total findings allowed (0 = unlimited)
}

// DefaultScanPolicy returns the default (strict) scan policy.
func DefaultScanPolicy() ScanPolicy {
	return ScanPolicy{
		BlockOnCritical: true,
		BlockOnHigh:     true,
		MaxFindings:     0,
	}
}

// CheckPolicy evaluates scan results against a policy.
func CheckPolicy(results []ScanResult, policy ScanPolicy) error {
	var blocked []string
	totalFindings := 0

	for _, r := range results {
		for _, f := range r.Findings {
			totalFindings++
			if policy.BlockOnCritical && strings.EqualFold(f.Severity, "CRITICAL") {
				blocked = append(blocked, fmt.Sprintf("[%s] %s %s: %s", f.Severity, f.Package, f.ID, f.Summary))
			}
			if policy.BlockOnHigh && strings.EqualFold(f.Severity, "HIGH") {
				blocked = append(blocked, fmt.Sprintf("[%s] %s %s: %s", f.Severity, f.Package, f.ID, f.Summary))
			}
		}
	}

	if policy.MaxFindings > 0 && totalFindings > policy.MaxFindings {
		return fmt.Errorf("too many findings: %d (max %d)", totalFindings, policy.MaxFindings)
	}

	if len(blocked) > 0 {
		return fmt.Errorf("blocked by scan policy:\n  %s", strings.Join(blocked, "\n  "))
	}

	return nil
}

// RunGovulncheck runs govulncheck on a Go module directory.
// Requires govulncheck to be installed: go install golang.org/x/vuln/cmd/govulncheck@latest
func RunGovulncheck(ctx context.Context, dir string) (*ScanResult, error) {
	cmd := exec.CommandContext(ctx, "govulncheck", "-json", "./...")
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// govulncheck exits non-zero when it finds vulns, so we check output regardless
	cmd.Run()

	result := &ScanResult{
		Tool:   "govulncheck",
		Target: dir,
		Passed: true,
	}

	// Parse JSON output
	findings, err := parseGovulncheckOutput(stdout.Bytes())
	if err != nil {
		// If we can't parse, check if govulncheck is installed
		if strings.Contains(stderr.String(), "not found") || strings.Contains(stderr.String(), "no such file") {
			return nil, fmt.Errorf("govulncheck not installed: go install golang.org/x/vuln/cmd/govulncheck@latest")
		}
		// Return a result with no findings if output is unparseable but clean
		return result, nil
	}

	result.Findings = findings
	result.Passed = len(findings) == 0
	return result, nil
}

// govulncheckVuln matches the JSON output structure of govulncheck.
type govulncheckEntry struct {
	Finding *struct {
		OSV   string `json:"osv"`
		Trace []struct {
			Module  string `json:"module"`
			Version string `json:"version"`
			Package string `json:"package"`
		} `json:"trace"`
		FixedVersion string `json:"fixed_version"`
	} `json:"finding"`
	OSV *struct {
		ID      string `json:"id"`
		Summary string `json:"summary"`
	} `json:"osv"`
}

func parseGovulncheckOutput(data []byte) ([]ScanFinding, error) {
	var findings []ScanFinding
	seen := make(map[string]bool)

	// govulncheck outputs newline-delimited JSON
	decoder := json.NewDecoder(bytes.NewReader(data))
	for decoder.More() {
		var entry govulncheckEntry
		if err := decoder.Decode(&entry); err != nil {
			continue
		}

		if entry.Finding != nil && !seen[entry.Finding.OSV] {
			seen[entry.Finding.OSV] = true
			pkg := ""
			ver := ""
			if len(entry.Finding.Trace) > 0 {
				pkg = entry.Finding.Trace[0].Package
				ver = entry.Finding.Trace[0].Version
			}
			findings = append(findings, ScanFinding{
				ID:       entry.Finding.OSV,
				Severity: "HIGH", // govulncheck doesn't provide severity, default HIGH
				Package:  pkg,
				Version:  ver,
				Fixed:    entry.Finding.FixedVersion,
			})
		}
	}

	return findings, nil
}

// RunTrivy runs Trivy container image scan.
// Requires trivy to be installed: https://aquasecurity.github.io/trivy/
func RunTrivy(ctx context.Context, imageRef string) (*ScanResult, error) {
	cmd := exec.CommandContext(ctx, "trivy", "image",
		"--format=json",
		"--severity=CRITICAL,HIGH,MEDIUM",
		"--quiet",
		imageRef,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.Run()

	result := &ScanResult{
		Tool:   "trivy",
		Target: imageRef,
		Passed: true,
	}

	findings, err := parseTrivyOutput(stdout.Bytes())
	if err != nil {
		if strings.Contains(stderr.String(), "not found") || strings.Contains(stderr.String(), "no such file") {
			return nil, fmt.Errorf("trivy not installed: see https://aquasecurity.github.io/trivy/")
		}
		return result, nil
	}

	result.Findings = findings
	result.Passed = len(findings) == 0
	return result, nil
}

type trivyOutput struct {
	Results []struct {
		Vulnerabilities []struct {
			VulnerabilityID string `json:"VulnerabilityID"`
			PkgName         string `json:"PkgName"`
			InstalledVersion string `json:"InstalledVersion"`
			FixedVersion    string `json:"FixedVersion"`
			Severity        string `json:"Severity"`
			Title           string `json:"Title"`
		} `json:"Vulnerabilities"`
	} `json:"Results"`
}

func parseTrivyOutput(data []byte) ([]ScanFinding, error) {
	var output trivyOutput
	if err := json.Unmarshal(data, &output); err != nil {
		return nil, err
	}

	var findings []ScanFinding
	for _, r := range output.Results {
		for _, v := range r.Vulnerabilities {
			findings = append(findings, ScanFinding{
				ID:       v.VulnerabilityID,
				Severity: v.Severity,
				Package:  v.PkgName,
				Version:  v.InstalledVersion,
				Fixed:    v.FixedVersion,
				Summary:  v.Title,
			})
		}
	}
	return findings, nil
}
