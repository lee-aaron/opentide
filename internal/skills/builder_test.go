package skills

import (
	"encoding/json"
	"testing"
)

func TestBuildOptionsValidation(t *testing.T) {
	_, err := BuildImage(t.Context(), BuildOptions{Tag: "", SkillDir: "."})
	if err == nil {
		t.Fatal("expected error for empty tag")
	}

	_, err = BuildImage(t.Context(), BuildOptions{Tag: "test:latest", SkillDir: ""})
	if err == nil {
		t.Fatal("expected error for empty skill dir")
	}
}

func TestMarshalBuildManifest(t *testing.T) {
	m := BuildManifest{
		SkillName:    "web-search",
		SkillVersion: "0.1.0",
		Image:        "opentide/skill-web-search:0.1.0",
		Digest:       "sha256:abc123",
		Platform:     "linux/amd64",
		Labels: map[string]string{
			"org.opentide.skill": "true",
		},
	}

	data, err := MarshalBuildManifest(m)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	// Round-trip
	var m2 BuildManifest
	if err := json.Unmarshal(data, &m2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if m2.Digest != "sha256:abc123" {
		t.Errorf("digest = %q", m2.Digest)
	}
	if m2.SkillName != "web-search" {
		t.Errorf("skill_name = %q", m2.SkillName)
	}
}
