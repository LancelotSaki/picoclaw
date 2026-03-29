package agent

import (
	"slices"
	"strings"
	"testing"

	"github.com/sipeed/picoclaw/pkg/config"
)

func TestAgentRegistry_ListAgentsBuildsStructuredDescriptors(t *testing.T) {
	mainWorkspace := setupWorkspace(t, map[string]string{
		"AGENT.md": `---
name: Main Frontmatter Name
description: Structured main agent
---
# Agent

Handle general requests.
`,
	})
	defer cleanupWorkspace(t, mainWorkspace)

	supportWorkspace := setupWorkspace(t, map[string]string{
		"AGENT.md": `# Agent

Handle support tickets carefully.
`,
		"SOUL.md": "# Soul\nStay calm and precise.",
	})
	defer cleanupWorkspace(t, supportWorkspace)

	cfg := testCfg([]config.AgentConfig{
		{ID: "main", Default: true, Name: "Configured Main", Workspace: mainWorkspace},
		{
			ID:        "support",
			Workspace: supportWorkspace,
			Model:     &config.AgentModelConfig{Primary: "support-model"},
		},
	})
	cfg.Tools.ReadFile.Enabled = true
	cfg.Tools.WriteFile.Enabled = true
	cfg.Channels.Telegram.Enabled = true
	cfg.Bindings = []config.AgentBinding{
		{
			AgentID: "support",
			Match: config.BindingMatch{
				Channel:   "telegram",
				AccountID: "*",
			},
		},
	}

	registry := NewAgentRegistry(cfg, &mockRegistryProvider{})

	descriptors := registry.ListAgents(mainWorkspace)
	if len(descriptors) != 2 {
		t.Fatalf("expected 2 descriptors, got %d", len(descriptors))
	}

	if descriptors[0].ID != "main" {
		t.Fatalf("expected current workspace agent first, got %q", descriptors[0].ID)
	}
	if descriptors[0].Name != "Configured Main" {
		t.Fatalf("expected config name to win, got %q", descriptors[0].Name)
	}
	if descriptors[0].Description != "Structured main agent" {
		t.Fatalf("expected frontmatter description, got %q", descriptors[0].Description)
	}
	if descriptors[0].Model != "gpt-4" {
		t.Fatalf("expected inherited model, got %q", descriptors[0].Model)
	}
	if !slices.Contains(descriptors[0].AvailableTools, "read_file") ||
		!slices.Contains(descriptors[0].AvailableTools, "write_file") {
		t.Fatalf("expected visible file tools in descriptor, got %v", descriptors[0].AvailableTools)
	}
	if !slices.Equal(descriptors[0].Channels, []string{"telegram"}) {
		t.Fatalf(
			"expected default agent to cover enabled telegram channel, got %v",
			descriptors[0].Channels,
		)
	}

	support, ok := registry.GetAgentDescriptor("support")
	if !ok || support == nil {
		t.Fatal("expected support descriptor lookup to succeed")
	}
	if support.Description != "Handle support tickets carefully." {
		t.Fatalf("expected AGENT body fallback description, got %q", support.Description)
	}
	if support.Model != "support-model" {
		t.Fatalf("expected explicit support model, got %q", support.Model)
	}
	if !slices.Equal(support.Channels, []string{"telegram"}) {
		t.Fatalf("expected support channel binding, got %v", support.Channels)
	}
}

func TestContextBuilder_BuildMessagesIncludesAgentDiscoverySection(t *testing.T) {
	mainWorkspace := setupWorkspace(t, map[string]string{
		"AGENT.md": `---
description: Main agent
---
# Agent

Generalist.
`,
	})
	defer cleanupWorkspace(t, mainWorkspace)

	researchWorkspace := setupWorkspace(t, map[string]string{
		"AGENT.md": `---
description: Research specialist
---
# Agent

Investigate deeply.
`,
	})
	defer cleanupWorkspace(t, researchWorkspace)

	cfg := testCfg([]config.AgentConfig{
		{ID: "main", Default: true, Workspace: mainWorkspace},
		{ID: "research", Workspace: researchWorkspace},
	})
	cfg.Tools.ReadFile.Enabled = true
	cfg.Tools.WriteFile.Enabled = true

	registry := NewAgentRegistry(cfg, &mockRegistryProvider{})
	mainAgent, ok := registry.GetAgent("main")
	if !ok || mainAgent == nil {
		t.Fatal("expected main agent")
	}

	messages := mainAgent.ContextBuilder.BuildMessages(
		nil,
		"",
		"delegate wisely",
		nil,
		"telegram",
		"chat-1",
		"",
		"",
	)
	if len(messages) == 0 {
		t.Fatal("expected messages")
	}

	systemPrompt := messages[0].Content
	if !strings.Contains(systemPrompt, "# Agent Discovery") {
		t.Fatalf("expected discovery section in system prompt, got %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, `"current_agent_id": "main"`) {
		t.Fatalf("expected current agent id in discovery section, got %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, `"id": "main"`) ||
		!strings.Contains(systemPrompt, `"id": "research"`) {
		t.Fatalf("expected self and peer descriptors in discovery section, got %q", systemPrompt)
	}
	if !strings.Contains(systemPrompt, `"available_tools": [`) ||
		!strings.Contains(systemPrompt, `"read_file"`) ||
		!strings.Contains(systemPrompt, `"write_file"`) {
		t.Fatalf("expected visible tool list in discovery section, got %q", systemPrompt)
	}
}

func TestContextBuilder_BuildMessagesOmitsAgentDiscoverySectionForSingleton(t *testing.T) {
	mainWorkspace := setupWorkspace(t, map[string]string{
		"AGENT.md": `---
description: Main agent
---
# Agent

Generalist.
`,
	})
	defer cleanupWorkspace(t, mainWorkspace)

	cfg := testCfg([]config.AgentConfig{
		{ID: "main", Default: true, Workspace: mainWorkspace},
	})
	cfg.Tools.ReadFile.Enabled = true

	registry := NewAgentRegistry(cfg, &mockRegistryProvider{})
	mainAgent, ok := registry.GetAgent("main")
	if !ok || mainAgent == nil {
		t.Fatal("expected main agent")
	}

	messages := mainAgent.ContextBuilder.BuildMessages(
		nil,
		"",
		"handle locally",
		nil,
		"telegram",
		"chat-1",
		"",
		"",
	)
	if len(messages) == 0 {
		t.Fatal("expected messages")
	}

	systemPrompt := messages[0].Content
	if strings.Contains(systemPrompt, "# Agent Discovery") {
		t.Fatalf("did not expect discovery section for singleton registry, got %q", systemPrompt)
	}
	if strings.Contains(systemPrompt, `"current_agent_id": "main"`) {
		t.Fatalf("did not expect discovery payload for singleton registry, got %q", systemPrompt)
	}
}
