package config

import (
	"testing"
	"time"

	"github.com/kunchenguid/no-mistakes/internal/types"
	"gopkg.in/yaml.v3"
)

func TestMerge_GlobalOnly(t *testing.T) {
	global := &GlobalConfig{
		Agent:     types.AgentClaude,
		CITimeout: 4 * time.Hour,
		LogLevel:  "info",
	}
	repo := &RepoConfig{}

	cfg := Merge(global, repo)
	if cfg.Agent != types.AgentClaude {
		t.Errorf("agent = %q, want %q", cfg.Agent, types.AgentClaude)
	}
	if cfg.CITimeout != 4*time.Hour {
		t.Errorf("ci_timeout = %v", cfg.CITimeout)
	}
}

func TestMerge_RepoOverridesAgent(t *testing.T) {
	global := &GlobalConfig{
		Agent:             types.AgentClaude,
		AgentPathOverride: map[string]string{"claude": "/usr/bin/claude"},
		CITimeout:         4 * time.Hour,
		LogLevel:          "info",
	}
	repo := &RepoConfig{
		Agent: types.AgentCodex,
		Commands: Commands{
			Test: "make test",
		},
	}

	cfg := Merge(global, repo)
	if cfg.Agent != types.AgentCodex {
		t.Errorf("agent = %q, want %q (repo override)", cfg.Agent, types.AgentCodex)
	}
	if cfg.AgentPathOverride["claude"] != "/usr/bin/claude" {
		t.Errorf("agent path override lost during merge")
	}
	if cfg.Commands.Test != "make test" {
		t.Errorf("test = %q", cfg.Commands.Test)
	}
	if cfg.CITimeout != 4*time.Hour {
		t.Errorf("ci_timeout = %v", cfg.CITimeout)
	}
}

func TestMerge_RepoOverridesCITimeout(t *testing.T) {
	// Regression test for a repo-local ci_timeout override being silently
	// ignored: RepoConfig had no ci_timeout field at all, and Merge() only
	// ever read CITimeout from global. See UnmarshalYAML's ci_timeout
	// handling and the "repo-local ci_timeout overrides the global default"
	// block in Merge for the fix.
	global := &GlobalConfig{
		Agent:     types.AgentClaude,
		CITimeout: 168 * time.Hour,
		LogLevel:  "info",
	}
	repo := &RepoConfig{
		CITimeout: 2 * time.Minute,
	}

	cfg := Merge(global, repo)
	if cfg.CITimeout != 2*time.Minute {
		t.Errorf("ci_timeout = %v, want 2m (repo override)", cfg.CITimeout)
	}
}

func TestMerge_RepoOverridesCITimeoutUnlimited(t *testing.T) {
	global := &GlobalConfig{
		Agent:     types.AgentClaude,
		CITimeout: 4 * time.Hour,
		LogLevel:  "info",
	}
	repo := &RepoConfig{
		CITimeout: CITimeoutUnlimited,
	}

	cfg := Merge(global, repo)
	if cfg.CITimeout != CITimeoutUnlimited {
		t.Errorf("ci_timeout = %v, want CITimeoutUnlimited (repo override)", cfg.CITimeout)
	}
}

func TestUnmarshalRepoConfig_CITimeout(t *testing.T) {
	yamlSrc := "ci_timeout: \"2m\"\n"
	var repo RepoConfig
	if err := yaml.Unmarshal([]byte(yamlSrc), &repo); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if repo.CITimeout != 2*time.Minute {
		t.Errorf("ci_timeout = %v, want 2m", repo.CITimeout)
	}
}

func TestUnmarshalRepoConfig_CITimeoutUnset(t *testing.T) {
	yamlSrc := "agent: claude\n"
	var repo RepoConfig
	if err := yaml.Unmarshal([]byte(yamlSrc), &repo); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if repo.CITimeout != 0 {
		t.Errorf("ci_timeout = %v, want 0 (unset)", repo.CITimeout)
	}
}

func TestMerge_RepoDoesNotOverrideWhenEmpty(t *testing.T) {
	global := &GlobalConfig{
		Agent:     types.AgentRovoDev,
		CITimeout: 2 * time.Hour,
		LogLevel:  "debug",
	}
	repo := &RepoConfig{
		// Agent is empty — should not override
		Commands: Commands{
			Lint: "eslint .",
		},
	}

	cfg := Merge(global, repo)
	if cfg.Agent != types.AgentRovoDev {
		t.Errorf("agent = %q, want %q (empty repo should not override)", cfg.Agent, types.AgentRovoDev)
	}
	if cfg.Commands.Lint != "eslint ." {
		t.Errorf("lint = %q", cfg.Commands.Lint)
	}
}

func TestMerge_AutoFixDefaults(t *testing.T) {
	global := &GlobalConfig{Agent: types.AgentClaude, CITimeout: 4 * time.Hour, LogLevel: "info"}
	repo := &RepoConfig{}

	cfg := Merge(global, repo)
	if cfg.AutoFix.Lint != 3 {
		t.Errorf("lint = %d, want 3", cfg.AutoFix.Lint)
	}
	if cfg.AutoFix.Test != 3 {
		t.Errorf("test = %d, want 3", cfg.AutoFix.Test)
	}
	if cfg.AutoFix.Review != 0 {
		t.Errorf("review = %d, want 0", cfg.AutoFix.Review)
	}
	if cfg.AutoFix.Document != 3 {
		t.Errorf("document = %d, want 3", cfg.AutoFix.Document)
	}
	if cfg.AutoFix.CI != 3 {
		t.Errorf("ci = %d, want 3", cfg.AutoFix.CI)
	}
	if cfg.AutoFix.Rebase != 3 {
		t.Errorf("rebase = %d, want 3", cfg.AutoFix.Rebase)
	}
}

func TestMerge_AutoFixGlobalOverridesDefaults(t *testing.T) {
	five := 5
	zero := 0
	global := &GlobalConfig{
		Agent:     types.AgentClaude,
		CITimeout: 4 * time.Hour,
		LogLevel:  "info",
		AutoFix:   AutoFixRaw{Lint: &five, CI: &zero},
	}
	repo := &RepoConfig{}

	cfg := Merge(global, repo)
	if cfg.AutoFix.Lint != 5 {
		t.Errorf("lint = %d, want 5 (global override)", cfg.AutoFix.Lint)
	}
	if cfg.AutoFix.Test != 3 {
		t.Errorf("test = %d, want 3 (default)", cfg.AutoFix.Test)
	}
	if cfg.AutoFix.CI != 0 {
		t.Errorf("ci =%d, want 0 (global override)", cfg.AutoFix.CI)
	}
	if cfg.AutoFix.Rebase != 3 {
		t.Errorf("rebase = %d, want 3 (default, no override)", cfg.AutoFix.Rebase)
	}
}

func TestMerge_AutoFixRepoOverridesGlobal(t *testing.T) {
	five := 5
	one := 1
	zero := 0
	global := &GlobalConfig{
		Agent:     types.AgentClaude,
		CITimeout: 4 * time.Hour,
		LogLevel:  "info",
		AutoFix:   AutoFixRaw{Lint: &five},
	}
	repo := &RepoConfig{
		AutoFix: AutoFixRaw{Lint: &one, Review: &zero},
	}

	cfg := Merge(global, repo)
	if cfg.AutoFix.Lint != 1 {
		t.Errorf("lint = %d, want 1 (repo override)", cfg.AutoFix.Lint)
	}
	if cfg.AutoFix.Review != 0 {
		t.Errorf("review = %d, want 0 (repo override)", cfg.AutoFix.Review)
	}
	if cfg.AutoFix.Test != 3 {
		t.Errorf("test = %d, want 3 (default, no override)", cfg.AutoFix.Test)
	}
}

func TestAutoFixLimit(t *testing.T) {
	cfg := &Config{
		AutoFix: AutoFix{Lint: 5, Test: 2, Review: 0, Document: 1, CI: 3, Rebase: 4},
	}
	tests := []struct {
		step types.StepName
		want int
	}{
		{types.StepLint, 5},
		{types.StepTest, 2},
		{types.StepReview, 0},
		{types.StepDocument, 1},
		{types.StepCI, 3},
		{types.StepRebase, 4},
		{types.StepPush, 0},
		{types.StepPR, 0},
	}
	for _, tt := range tests {
		got := cfg.AutoFixLimit(tt.step)
		if got != tt.want {
			t.Errorf("AutoFixLimit(%q) = %d, want %d", tt.step, got, tt.want)
		}
	}
}
