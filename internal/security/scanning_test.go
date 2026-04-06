package security

import (
	"testing"
)

func TestCheckPolicyClean(t *testing.T) {
	results := []ScanResult{
		{Tool: "govulncheck", Passed: true},
		{Tool: "trivy", Passed: true},
	}
	if err := CheckPolicy(results, DefaultScanPolicy()); err != nil {
		t.Errorf("expected clean pass, got: %v", err)
	}
}

func TestCheckPolicyBlocksCritical(t *testing.T) {
	results := []ScanResult{
		{
			Tool:   "trivy",
			Passed: false,
			Findings: []ScanFinding{
				{ID: "CVE-2026-1234", Severity: "CRITICAL", Package: "openssl", Summary: "RCE"},
			},
		},
	}
	err := CheckPolicy(results, DefaultScanPolicy())
	if err == nil {
		t.Fatal("expected block on critical finding")
	}
}

func TestCheckPolicyBlocksHigh(t *testing.T) {
	results := []ScanResult{
		{
			Tool:   "govulncheck",
			Passed: false,
			Findings: []ScanFinding{
				{ID: "GO-2026-0001", Severity: "HIGH", Package: "net/http"},
			},
		},
	}
	err := CheckPolicy(results, DefaultScanPolicy())
	if err == nil {
		t.Fatal("expected block on high finding")
	}
}

func TestCheckPolicyAllowsMedium(t *testing.T) {
	results := []ScanResult{
		{
			Tool:   "trivy",
			Passed: false,
			Findings: []ScanFinding{
				{ID: "CVE-2026-9999", Severity: "MEDIUM", Package: "curl"},
			},
		},
	}
	if err := CheckPolicy(results, DefaultScanPolicy()); err != nil {
		t.Errorf("medium should not be blocked by default: %v", err)
	}
}

func TestCheckPolicyMaxFindings(t *testing.T) {
	results := []ScanResult{
		{
			Tool:   "trivy",
			Passed: false,
			Findings: []ScanFinding{
				{ID: "CVE-1", Severity: "MEDIUM", Package: "a"},
				{ID: "CVE-2", Severity: "MEDIUM", Package: "b"},
				{ID: "CVE-3", Severity: "MEDIUM", Package: "c"},
			},
		},
	}
	policy := ScanPolicy{MaxFindings: 2}
	err := CheckPolicy(results, policy)
	if err == nil {
		t.Fatal("expected block on max findings exceeded")
	}
}

func TestParseGovulncheckOutput(t *testing.T) {
	// Simulated govulncheck JSON output
	data := `{"finding":{"osv":"GO-2026-0001","trace":[{"module":"stdlib","version":"go1.21.0","package":"net/http"}],"fixed_version":"go1.21.1"}}
{"osv":{"id":"GO-2026-0001","summary":"HTTP smuggling in net/http"}}
`
	findings, err := parseGovulncheckOutput([]byte(data))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].ID != "GO-2026-0001" {
		t.Errorf("id = %q", findings[0].ID)
	}
	if findings[0].Fixed != "go1.21.1" {
		t.Errorf("fixed = %q", findings[0].Fixed)
	}
}

func TestParseTrivyOutput(t *testing.T) {
	data := `{"Results":[{"Vulnerabilities":[{"VulnerabilityID":"CVE-2026-1234","PkgName":"openssl","InstalledVersion":"3.0.0","FixedVersion":"3.0.1","Severity":"CRITICAL","Title":"Buffer overflow"}]}]}`
	findings, err := parseTrivyOutput([]byte(data))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].ID != "CVE-2026-1234" {
		t.Errorf("id = %q", findings[0].ID)
	}
	if findings[0].Severity != "CRITICAL" {
		t.Errorf("severity = %q", findings[0].Severity)
	}
}

func TestParseGovulncheckOutputEmpty(t *testing.T) {
	findings, err := parseGovulncheckOutput([]byte(""))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}
