package skills

import (
	"strings"
	"testing"
)

func TestValidateEgressEntry(t *testing.T) {
	tests := []struct {
		entry   string
		wantErr bool
	}{
		{"api.brave.com:443", false},
		{"example.com:8080", false},
		{"10.0.0.1:443", false},
		{"*", true},
		{"no-port", true},
		{":443", true},
		{"host:", true},
	}

	for _, tt := range tests {
		t.Run(tt.entry, func(t *testing.T) {
			err := ValidateEgressEntry(tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEgressEntry(%q) error = %v, wantErr = %v", tt.entry, err, tt.wantErr)
			}
		})
	}
}

func TestResolveEgressRules(t *testing.T) {
	// Use localhost which always resolves
	rules, err := ResolveEgressRules([]string{"localhost:443"})
	if err != nil {
		t.Fatalf("ResolveEgressRules failed: %v", err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Host != "localhost" {
		t.Errorf("host = %q, want localhost", rules[0].Host)
	}
	if rules[0].Port != "443" {
		t.Errorf("port = %q, want 443", rules[0].Port)
	}
	if len(rules[0].IPs) == 0 {
		t.Error("expected at least one resolved IP")
	}
}

func TestResolveEgressRulesInvalidEntry(t *testing.T) {
	_, err := ResolveEgressRules([]string{"no-port"})
	if err == nil {
		t.Fatal("expected error for invalid entry")
	}
}

func TestResolveEgressRulesUnresolvable(t *testing.T) {
	_, err := ResolveEgressRules([]string{"this-host-definitely-does-not-exist-xyz.invalid:443"})
	if err == nil {
		t.Fatal("expected error for unresolvable host")
	}
}

func TestGenerateIptablesRules(t *testing.T) {
	rules := []EgressRule{
		{Host: "api.brave.com", Port: "443", IPs: []string{"1.2.3.4", "5.6.7.8"}},
		{Host: "api.google.com", Port: "443", IPs: []string{"9.10.11.12"}},
	}

	cmds := GenerateIptablesRules(rules)

	// Default policy: drop
	if cmds[0] != "iptables -P OUTPUT DROP" {
		t.Errorf("first rule should be DROP policy, got %q", cmds[0])
	}

	// Check that allowed IPs appear
	joined := strings.Join(cmds, "\n")
	for _, ip := range []string{"1.2.3.4", "5.6.7.8", "9.10.11.12"} {
		if !strings.Contains(joined, ip) {
			t.Errorf("iptables rules should contain IP %s", ip)
		}
	}

	// DNS should be allowed
	if !strings.Contains(joined, "--dport 53") {
		t.Error("iptables rules should allow DNS (port 53)")
	}

	// Loopback should be allowed
	if !strings.Contains(joined, "-o lo -j ACCEPT") {
		t.Error("iptables rules should allow loopback")
	}
}

func TestGenerateIptablesRulesEmpty(t *testing.T) {
	cmds := GenerateIptablesRules(nil)
	// Should still have the base rules (DROP, loopback, established, DNS)
	if len(cmds) < 4 {
		t.Errorf("expected at least 4 base rules, got %d", len(cmds))
	}
}

func TestNetworkArgsNoEgress(t *testing.T) {
	ec := NewEgressController()
	args := ec.NetworkArgs("test-skill", nil)
	if len(args) != 1 || args[0] != "--network=none" {
		t.Errorf("expected --network=none, got %v", args)
	}
}

func TestNetworkArgsWithEgress(t *testing.T) {
	ec := NewEgressController()
	// Manually register a network
	ec.mu.Lock()
	ec.networks["test-skill"] = "opentide-test-skill"
	ec.mu.Unlock()

	args := ec.NetworkArgs("test-skill", []string{"api.example.com:443"})
	if len(args) != 1 || args[0] != "--network=opentide-test-skill" {
		t.Errorf("expected --network=opentide-test-skill, got %v", args)
	}
}

func TestSanitizeNetName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"web-search", "web-search"},
		{"Web_Search", "web-search"},
		{"my.skill.v2", "my-skill-v2"},
		{"UPPERCASE", "uppercase"},
	}
	for _, tt := range tests {
		got := sanitizeNetName(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeNetName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
