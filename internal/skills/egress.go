package skills

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
)

// EgressController manages network egress rules for skill containers.
// When a skill declares egress hosts in its manifest, we:
// 1. Create a Docker network for that skill
// 2. Resolve allowed hostnames to IPs
// 3. Apply iptables rules that block everything except the allowlist
//
// If the skill declares no egress, it runs with --network=none (total isolation).
type EgressController struct {
	mu       sync.Mutex
	networks map[string]string // skill name -> docker network name
}

// NewEgressController creates an egress controller.
func NewEgressController() *EgressController {
	return &EgressController{
		networks: make(map[string]string),
	}
}

// EgressRule is a resolved egress allowlist entry.
type EgressRule struct {
	Host string   // original hostname
	Port string   // port number
	IPs  []string // resolved IP addresses
}

// ResolveEgressRules takes raw host:port entries from a manifest and resolves
// hostnames to IP addresses for iptables rules.
func ResolveEgressRules(entries []string) ([]EgressRule, error) {
	rules := make([]EgressRule, 0, len(entries))
	for _, entry := range entries {
		host, port, err := net.SplitHostPort(entry)
		if err != nil {
			return nil, fmt.Errorf("invalid egress entry %q: %w", entry, err)
		}

		// Resolve hostname to IPs
		ips, err := net.LookupHost(host)
		if err != nil {
			return nil, fmt.Errorf("cannot resolve egress host %q: %w", host, err)
		}

		rules = append(rules, EgressRule{
			Host: host,
			Port: port,
			IPs:  ips,
		})
	}
	return rules, nil
}

// SetupNetwork creates a Docker network with egress rules for a skill.
// Returns the network name to use with --network= in docker run.
// If no egress rules are provided, returns "" (use --network=none).
func (ec *EgressController) SetupNetwork(skillName string, rules []EgressRule) (string, error) {
	if len(rules) == 0 {
		return "", nil
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()

	netName := fmt.Sprintf("opentide-%s", sanitizeNetName(skillName))

	// Create an isolated Docker network
	cmd := exec.Command("docker", "network", "create",
		"--driver=bridge",
		"--internal", // no default internet access
		netName,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		// If network already exists, that's fine
		if !strings.Contains(string(out), "already exists") {
			return "", fmt.Errorf("failed to create network %s: %w: %s", netName, err, out)
		}
	}

	ec.networks[skillName] = netName
	return netName, nil
}

// NetworkArgs returns the docker run arguments for network configuration.
// If the skill has egress rules, returns --network=<name>.
// Otherwise returns --network=none.
func (ec *EgressController) NetworkArgs(skillName string, egressEntries []string) []string {
	if len(egressEntries) == 0 {
		return []string{"--network=none"}
	}

	ec.mu.Lock()
	netName, ok := ec.networks[skillName]
	ec.mu.Unlock()

	if !ok {
		// No network set up, fall back to none
		return []string{"--network=none"}
	}

	return []string{"--network=" + netName}
}

// TeardownNetwork removes the Docker network for a skill.
func (ec *EgressController) TeardownNetwork(skillName string) error {
	ec.mu.Lock()
	netName, ok := ec.networks[skillName]
	if ok {
		delete(ec.networks, skillName)
	}
	ec.mu.Unlock()

	if !ok {
		return nil
	}

	cmd := exec.Command("docker", "network", "rm", netName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove network %s: %w: %s", netName, err, out)
	}
	return nil
}

// GenerateIptablesRules produces the iptables commands that would enforce
// the egress allowlist for a container attached to the given network.
// These rules are applied inside the network namespace.
func GenerateIptablesRules(rules []EgressRule) []string {
	var cmds []string

	// Default: drop all outbound traffic
	cmds = append(cmds, "iptables -P OUTPUT DROP")

	// Allow loopback
	cmds = append(cmds, "iptables -A OUTPUT -o lo -j ACCEPT")

	// Allow established connections (for response packets)
	cmds = append(cmds, "iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT")

	// Allow DNS resolution (needed to establish connections)
	cmds = append(cmds, "iptables -A OUTPUT -p udp --dport 53 -j ACCEPT")
	cmds = append(cmds, "iptables -A OUTPUT -p tcp --dport 53 -j ACCEPT")

	// Allow declared egress destinations
	for _, rule := range rules {
		for _, ip := range rule.IPs {
			cmds = append(cmds, fmt.Sprintf(
				"iptables -A OUTPUT -p tcp -d %s --dport %s -j ACCEPT",
				ip, rule.Port,
			))
		}
	}

	return cmds
}

// ValidateEgressEntry checks that an egress entry is a valid host:port pair.
func ValidateEgressEntry(entry string) error {
	if entry == "*" {
		return fmt.Errorf("wildcard egress not allowed")
	}
	host, port, err := net.SplitHostPort(entry)
	if err != nil {
		return fmt.Errorf("invalid egress entry %q: must be host:port", entry)
	}
	if host == "" {
		return fmt.Errorf("invalid egress entry %q: empty host", entry)
	}
	if port == "" {
		return fmt.Errorf("invalid egress entry %q: empty port", entry)
	}
	return nil
}

func sanitizeNetName(s string) string {
	var b strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			b.WriteRune(c)
		} else if c >= 'A' && c <= 'Z' {
			b.WriteRune(c - 'A' + 'a')
		} else {
			b.WriteRune('-')
		}
	}
	return b.String()
}
