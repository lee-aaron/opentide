package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/opentide/opentide/internal/registry"
	"github.com/opentide/opentide/internal/security"
	"github.com/opentide/opentide/pkg/skillspec"
)

func cmdSkill() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: tide-cli skill <new|list|verify|sign|keygen|publish|search|install>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "new":
		cmdSkillNew()
	case "list":
		cmdSkillList()
	case "verify":
		cmdSkillVerify()
	case "sign":
		cmdSkillSign()
	case "keygen":
		cmdSkillKeygen()
	case "publish":
		cmdSkillPublish()
	case "search":
		cmdSkillSearch()
	case "install":
		cmdSkillInstall()
	default:
		fmt.Fprintf(os.Stderr, "Unknown skill command: %s\n", os.Args[2])
		os.Exit(1)
	}
}

func cmdSkillList() {
	dir := "skills"
	if len(os.Args) > 3 {
		dir = os.Args[3]
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read skills directory: %v\n", err)
		os.Exit(1)
	}

	found := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(dir, entry.Name(), "skill.yaml")
		m, err := skillspec.LoadManifest(manifestPath)
		if err != nil {
			fmt.Printf("  %-20s ERROR: %v\n", entry.Name(), err)
			continue
		}
		fmt.Printf("  %-20s v%-10s %s\n", m.Name, m.Version, m.Description)
		if m.Triggers.ToolName != "" {
			fmt.Printf("  %-20s tool: %s\n", "", m.Triggers.ToolName)
		}
		if len(m.Security.Egress) > 0 {
			fmt.Printf("  %-20s egress: %v\n", "", m.Security.Egress)
		} else {
			fmt.Printf("  %-20s egress: none (fully isolated)\n", "")
		}
		fmt.Println()
		found++
	}

	if found == 0 {
		fmt.Println("No skills found.")
	} else {
		fmt.Printf("%d skill(s) found.\n", found)
	}
}

func cmdSkillVerify() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: tide-cli skill verify <signed-manifest.yaml>")
		os.Exit(1)
	}

	path := os.Args[3]
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read file: %v\n", err)
		os.Exit(1)
	}

	var signed skillspec.SignedManifest
	if err := yaml.Unmarshal(data, &signed); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid signed manifest: %v\n", err)
		os.Exit(1)
	}

	if signed.Signature.Signature == "" {
		fmt.Fprintln(os.Stderr, "FAILED: manifest is not signed")
		os.Exit(1)
	}

	if err := security.VerifyManifest(&signed); err != nil {
		fmt.Fprintf(os.Stderr, "FAILED: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("VERIFIED: %s v%s\n", signed.Manifest.Name, signed.Manifest.Version)
	fmt.Printf("  Signed by: %s...%s\n", signed.Signature.PublicKey[:16], signed.Signature.PublicKey[len(signed.Signature.PublicKey)-8:])
	fmt.Printf("  Signed at: %s\n", signed.Signature.SignedAt)
}

func cmdSkillSign() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: tide-cli skill sign <skill.yaml> --key <private-key-file>")
		os.Exit(1)
	}

	manifestPath := os.Args[3]
	keyPath := ""
	for i := 4; i < len(os.Args); i++ {
		if os.Args[i] == "--key" && i+1 < len(os.Args) {
			keyPath = os.Args[i+1]
			break
		}
	}
	if keyPath == "" {
		fmt.Fprintln(os.Stderr, "Missing --key flag")
		os.Exit(1)
	}

	m, err := skillspec.LoadManifest(manifestPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid manifest: %v\n", err)
		os.Exit(1)
	}

	keyHex, err := os.ReadFile(keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read key file: %v\n", err)
		os.Exit(1)
	}

	privKey, err := security.LoadPrivateKey(string(keyHex))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Invalid private key: %v\n", err)
		os.Exit(1)
	}

	signed, err := security.SignManifest(m, privKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Signing failed: %v\n", err)
		os.Exit(1)
	}

	outPath := manifestPath + ".signed"
	data, err := yaml.Marshal(signed)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Marshal failed: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Write failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Signed manifest written to %s\n", outPath)
}

func cmdSkillKeygen() {
	kp, err := security.GenerateKeyPair()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Key generation failed: %v\n", err)
		os.Exit(1)
	}

	pubFile := "signing.pub"
	privFile := "signing.key"

	if len(os.Args) > 3 {
		base := os.Args[3]
		pubFile = base + ".pub"
		privFile = base + ".key"
	}

	if err := os.WriteFile(pubFile, []byte(kp.PublicKeyHex()), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write public key: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(privFile, []byte(kp.PrivateKeyHex()), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write private key: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Key pair generated:\n")
	fmt.Printf("  Public key:  %s\n", pubFile)
	fmt.Printf("  Private key: %s (keep this secret!)\n", privFile)

	// Also print as JSON for programmatic use
	info := map[string]string{
		"public_key":  kp.PublicKeyHex(),
		"public_file": pubFile,
		"private_file": privFile,
	}
	jsonBytes, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(jsonBytes))
}

const defaultRegistryURL = "http://localhost:8081"

func registryURL() string {
	if v := os.Getenv("OPENTIDE_REGISTRY"); v != "" {
		return strings.TrimRight(v, "/")
	}
	return defaultRegistryURL
}

func cmdSkillPublish() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: tide-cli skill publish <signed-manifest.yaml> --image <image-ref>")
		os.Exit(1)
	}

	path := os.Args[3]
	imageRef := ""
	for i := 4; i < len(os.Args); i++ {
		if os.Args[i] == "--image" && i+1 < len(os.Args) {
			imageRef = os.Args[i+1]
			break
		}
	}
	if imageRef == "" {
		fmt.Fprintln(os.Stderr, "Missing --image flag")
		os.Exit(1)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Cannot read file: %v\n", err)
		os.Exit(1)
	}

	var signed skillspec.SignedManifest
	if err := yaml.Unmarshal(data, &signed); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid signed manifest: %v\n", err)
		os.Exit(1)
	}

	client := registry.NewClient(registryURL())
	if err := client.Publish(context.Background(), &signed, imageRef); err != nil {
		fmt.Fprintf(os.Stderr, "Publish failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Published %s v%s to %s\n", signed.Manifest.Name, signed.Manifest.Version, registryURL())
}

func cmdSkillSearch() {
	term := ""
	author := ""
	for i := 3; i < len(os.Args); i++ {
		if os.Args[i] == "--author" && i+1 < len(os.Args) {
			author = os.Args[i+1]
			i++
		} else if !strings.HasPrefix(os.Args[i], "--") {
			term = os.Args[i]
		}
	}

	client := registry.NewClient(registryURL())
	result, err := client.Search(context.Background(), term, author)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Search failed: %v\n", err)
		os.Exit(1)
	}

	if result.Total == 0 {
		fmt.Println("No skills found.")
		return
	}

	for _, e := range result.Entries {
		fmt.Printf("  %-20s v%-10s by %-15s %s\n", e.Name, e.Version, e.Author, e.Description)
		fmt.Printf("  %-20s downloads: %d\n", "", e.Downloads)
		fmt.Println()
	}
	fmt.Printf("%d skill(s) found.\n", result.Total)
}

func cmdSkillNew() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: tide-cli skill new <name>")
		os.Exit(1)
	}

	name := os.Args[3]
	dir := filepath.Join("skills", name)

	if _, err := os.Stat(dir); err == nil {
		fmt.Fprintf(os.Stderr, "Skill directory already exists: %s\n", dir)
		os.Exit(1)
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Cannot create directory: %v\n", err)
		os.Exit(1)
	}

	// skill.yaml
	manifest := fmt.Sprintf(`name: %s
version: 0.1.0
description: TODO - describe what this skill does
author: opentide
license: Apache-2.0

security:
  egress: []
  filesystem: read-only
  max_memory: 128Mi
  max_cpu: "0.5"
  timeout: 30s

triggers:
  tool_name: %s
  keywords:
    - TODO

runtime:
  image: opentide/skill-%s:0.1.0
  dockerfile: Dockerfile
  entrypoint: /skill
`, name, strings.ReplaceAll(name, "-", "_"), name)

	// main.go
	mainGo := fmt.Sprintf(`package main

import (
	"encoding/json"
	"io"
	"os"
)

type Input struct {
	Arguments map[string]any `+"`"+`json:"arguments"`+"`"+`
}

type Output struct {
	Content string `+"`"+`json:"content"`+"`"+`
	Error   string `+"`"+`json:"error,omitempty"`+"`"+`
}

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		writeError("failed to read input: " + err.Error())
		return
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		writeError("invalid input JSON: " + err.Error())
		return
	}

	query, _ := input.Arguments["query"].(string)
	if query == "" {
		writeError("missing 'query' argument")
		return
	}

	// TODO: implement skill logic for %s
	writeOutput("Hello from %s! Query: " + query)
}

func writeOutput(content string) {
	json.NewEncoder(os.Stdout).Encode(Output{Content: content})
}

func writeError(msg string) {
	json.NewEncoder(os.Stdout).Encode(Output{Error: msg})
}
`, name, name)

	// go.mod
	goMod := fmt.Sprintf(`module github.com/opentide/opentide/skills/%s

go 1.23
`, name)

	// Dockerfile
	dockerfile := fmt.Sprintf(`FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY main.go .
COPY go.mod .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /skill .

FROM alpine:3.21
RUN adduser -D -u 1000 skill
COPY --from=builder /skill /skill
USER skill
ENTRYPOINT ["/skill"]
`)

	files := map[string]string{
		"skill.yaml": manifest,
		"main.go":    mainGo,
		"go.mod":     goMod,
		"Dockerfile": dockerfile,
	}

	for filename, content := range files {
		path := filepath.Join(dir, filename)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to write %s: %v\n", path, err)
			os.Exit(1)
		}
	}

	fmt.Printf("Skill scaffolded in %s/\n", dir)
	fmt.Println("Files created:")
	fmt.Println("  skill.yaml  - manifest (update description, triggers, egress)")
	fmt.Println("  main.go     - skill logic (reads JSON from stdin, writes JSON to stdout)")
	fmt.Println("  go.mod      - Go module")
	fmt.Println("  Dockerfile  - container build")
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Edit skills/%s/main.go to implement your skill logic\n", name)
	fmt.Printf("  2. Update skills/%s/skill.yaml with proper description and triggers\n", name)
	fmt.Printf("  3. Test: cd skills/%s && go build -o /dev/null .\n", name)
}

func cmdSkillInstall() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "Usage: tide-cli skill install <name> [--version <version>]")
		os.Exit(1)
	}

	name := os.Args[3]
	version := ""
	for i := 4; i < len(os.Args); i++ {
		if os.Args[i] == "--version" && i+1 < len(os.Args) {
			version = os.Args[i+1]
			break
		}
	}

	client := registry.NewClient(registryURL())
	entry, err := client.Install(context.Background(), name, version)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Install failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installed %s v%s\n", entry.Name, entry.Version)
	fmt.Printf("  Image: %s\n", entry.ImageRef)
	fmt.Printf("  Author: %s\n", entry.Author)

	// Write the manifest locally
	skillDir := filepath.Join("skills", entry.Name)
	os.MkdirAll(skillDir, 0755)

	if entry.Signed != nil {
		data, _ := yaml.Marshal(entry.Signed.Manifest)
		os.WriteFile(filepath.Join(skillDir, "skill.yaml"), data, 0644)
		fmt.Printf("  Manifest written to %s/skill.yaml\n", skillDir)
	}
}
