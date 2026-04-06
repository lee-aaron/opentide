package skills

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// BuildResult is the output of building a skill container image.
type BuildResult struct {
	Image  string `json:"image"`  // image tag
	Digest string `json:"digest"` // sha256 digest for pinning
	Size   int64  `json:"size"`   // image size in bytes
}

// BuildOptions configures the skill image build.
type BuildOptions struct {
	SkillDir   string // directory containing Dockerfile and skill.yaml
	Tag        string // image tag (e.g. "opentide/skill-web-search:0.1.0")
	NoCache    bool   // disable Docker build cache for reproducibility
	Platform   string // target platform (e.g. "linux/amd64")
}

// BuildImage builds a skill container image from its Dockerfile.
// Returns the image digest for pinning in the registry.
func BuildImage(ctx context.Context, opts BuildOptions) (*BuildResult, error) {
	if opts.Tag == "" {
		return nil, fmt.Errorf("image tag is required")
	}
	if opts.SkillDir == "" {
		return nil, fmt.Errorf("skill directory is required")
	}

	args := []string{"build", "-t", opts.Tag}

	if opts.NoCache {
		args = append(args, "--no-cache")
	}

	if opts.Platform != "" {
		args = append(args, "--platform", opts.Platform)
	}

	// Build args for reproducibility
	args = append(args,
		"--label=org.opentide.skill=true",
		"-f", "Dockerfile",
		".",
	)

	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Dir = opts.SkillDir

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("docker build failed: %w\n%s", err, stderr.String())
	}

	// Get the image digest
	digest, err := getImageDigest(ctx, opts.Tag)
	if err != nil {
		return nil, fmt.Errorf("failed to get image digest: %w", err)
	}

	size, _ := getImageSize(ctx, opts.Tag)

	return &BuildResult{
		Image:  opts.Tag,
		Digest: digest,
		Size:   size,
	}, nil
}

// getImageDigest returns the sha256 digest of a local Docker image.
func getImageDigest(ctx context.Context, tag string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format={{index .RepoDigests 0}}", tag)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		// Fallback: use the image ID
		cmd2 := exec.CommandContext(ctx, "docker", "inspect", "--format={{.Id}}", tag)
		var stdout2 bytes.Buffer
		cmd2.Stdout = &stdout2
		if err2 := cmd2.Run(); err2 != nil {
			return "", fmt.Errorf("cannot inspect image: %w", err2)
		}
		return strings.TrimSpace(stdout2.String()), nil
	}

	return strings.TrimSpace(stdout.String()), nil
}

// getImageSize returns the size of a Docker image in bytes.
func getImageSize(ctx context.Context, tag string) (int64, error) {
	cmd := exec.CommandContext(ctx, "docker", "inspect", "--format={{.Size}}", tag)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return 0, err
	}

	var size int64
	fmt.Sscanf(strings.TrimSpace(stdout.String()), "%d", &size)
	return size, nil
}

// VerifyDigest checks that a local image matches the expected digest.
// This is used during install to ensure the image hasn't been tampered with.
func VerifyDigest(ctx context.Context, tag, expectedDigest string) error {
	actual, err := getImageDigest(ctx, tag)
	if err != nil {
		return fmt.Errorf("cannot get digest for %s: %w", tag, err)
	}

	if actual != expectedDigest {
		return fmt.Errorf("digest mismatch for %s: expected %s, got %s (possible tampering)", tag, expectedDigest, actual)
	}
	return nil
}

// PushImage pushes a built image to a container registry.
func PushImage(ctx context.Context, tag string) error {
	cmd := exec.CommandContext(ctx, "docker", "push", tag)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker push failed: %w\n%s", err, stderr.String())
	}
	return nil
}

// PullImage pulls a skill image from a container registry.
func PullImage(ctx context.Context, imageRef string) error {
	cmd := exec.CommandContext(ctx, "docker", "pull", imageRef)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker pull failed: %w\n%s", err, stderr.String())
	}
	return nil
}

// ImageExists checks if a Docker image exists locally.
func ImageExists(ctx context.Context, tag string) bool {
	cmd := exec.CommandContext(ctx, "docker", "image", "inspect", tag)
	return cmd.Run() == nil
}

// BuildManifestJSON generates a build manifest JSON for reproducibility auditing.
// This records the exact inputs and outputs of a build.
type BuildManifest struct {
	SkillName    string            `json:"skill_name"`
	SkillVersion string            `json:"skill_version"`
	Image        string            `json:"image"`
	Digest       string            `json:"digest"`
	Platform     string            `json:"platform"`
	BuildArgs    map[string]string `json:"build_args,omitempty"`
	Labels       map[string]string `json:"labels,omitempty"`
}

// MarshalBuildManifest serializes a build manifest to JSON.
func MarshalBuildManifest(m BuildManifest) ([]byte, error) {
	return json.MarshalIndent(m, "", "  ")
}
