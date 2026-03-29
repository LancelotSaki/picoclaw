package agent

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
	"github.com/sipeed/picoclaw/pkg/routing"
)

// AgentDescriptor is the structured discovery payload injected into each
// agent's system prompt so the LLM can make concrete delegation decisions.
type AgentDescriptor struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Description    string   `json:"description"`
	Model          string   `json:"model"`
	AvailableTools []string `json:"available_tools"`
	Channels       []string `json:"channels"`
}

// ListAgents returns structured descriptors for every agent in the current
// PicoClaw instance. The current workspace, when provided, is used only to
// order the matching agent first for prompt readability.
func (r *AgentRegistry) ListAgents(workspace string) []AgentDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := make([]string, 0, len(r.agents))
	for id := range r.agents {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	selfWorkspace := cleanWorkspacePath(workspace)
	descriptors := make([]AgentDescriptor, 0, len(ids))
	for _, id := range ids {
		agent := r.agents[id]
		if agent == nil {
			continue
		}
		descriptors = append(descriptors, r.buildAgentDescriptorLocked(agent))
	}

	if selfWorkspace == "" {
		return descriptors
	}

	sort.SliceStable(descriptors, func(i, j int) bool {
		leftSelf := cleanWorkspacePath(
			r.workspaceForAgentIDLocked(descriptors[i].ID),
		) == selfWorkspace
		rightSelf := cleanWorkspacePath(
			r.workspaceForAgentIDLocked(descriptors[j].ID),
		) == selfWorkspace
		if leftSelf != rightSelf {
			return leftSelf
		}
		return descriptors[i].ID < descriptors[j].ID
	})

	return descriptors
}

// GetAgentDescriptor returns the structured discovery payload for one agent.
func (r *AgentRegistry) GetAgentDescriptor(agentID string) (*AgentDescriptor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id := routing.NormalizeAgentID(agentID)
	agent, ok := r.agents[id]
	if !ok || agent == nil {
		return nil, false
	}

	descriptor := r.buildAgentDescriptorLocked(agent)
	return &descriptor, true
}

func (r *AgentRegistry) buildAgentDescriptorLocked(agent *AgentInstance) AgentDescriptor {
	definition := loadAgentDefinition(agent.Workspace)
	name := strings.TrimSpace(agent.Name)
	if name == "" && definition.Agent != nil {
		name = strings.TrimSpace(definition.Agent.Frontmatter.Name)
	}
	if name == "" {
		name = agent.ID
	}

	return AgentDescriptor{
		ID:             agent.ID,
		Name:           name,
		Description:    agentDescriptionFromDefinition(definition),
		Model:          strings.TrimSpace(agent.Model),
		AvailableTools: visibleToolNames(agent),
		Channels:       r.channelsForAgentLocked(agent.ID),
	}
}

func visibleToolNames(agent *AgentInstance) []string {
	if agent == nil || agent.Tools == nil {
		return []string{}
	}

	defs := agent.Tools.ToProviderDefs()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		name := strings.TrimSpace(def.Function.Name)
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if names == nil {
		return []string{}
	}
	return names
}

func agentDescriptionFromDefinition(definition AgentContextDefinition) string {
	if definition.Agent != nil {
		if desc := strings.TrimSpace(definition.Agent.Frontmatter.Description); desc != "" {
			return desc
		}
		if desc := firstMeaningfulParagraph(definition.Agent.Body); desc != "" {
			return desc
		}
	}
	if definition.Soul != nil {
		if desc := firstMeaningfulParagraph(definition.Soul.Content); desc != "" {
			return desc
		}
	}
	return ""
}

func firstMeaningfulParagraph(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	paragraphs := strings.Split(content, "\n\n")
	for _, paragraph := range paragraphs {
		lines := strings.Split(paragraph, "\n")
		parts := make([]string, 0, len(lines))
		inFence := false
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") {
				inFence = !inFence
				continue
			}
			if inFence || trimmed == "" {
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				trimmed = strings.TrimSpace(trimmed[2:])
			}
			parts = append(parts, trimmed)
		}
		if len(parts) == 0 {
			continue
		}
		return strings.Join(parts, " ")
	}
	return ""
}

func (r *AgentRegistry) channelsForAgentLocked(agentID string) []string {
	channels := make(map[string]struct{})

	if defaultID := r.defaultAgentIDLocked(); defaultID != "" && defaultID == agentID {
		for _, channel := range enabledChannels(r.cfg) {
			channels[channel] = struct{}{}
		}
	}

	if r.cfg != nil {
		for _, binding := range r.cfg.Bindings {
			if routing.NormalizeAgentID(binding.AgentID) != agentID {
				continue
			}
			channel := strings.ToLower(strings.TrimSpace(binding.Match.Channel))
			if channel == "" {
				continue
			}
			channels[channel] = struct{}{}
		}
	}

	if len(channels) == 0 {
		return []string{}
	}

	result := make([]string, 0, len(channels))
	for channel := range channels {
		result = append(result, channel)
	}
	sort.Strings(result)
	return result
}

func enabledChannels(cfg *config.Config) []string {
	if cfg == nil {
		return []string{}
	}

	enabled := make([]string, 0, 16)
	if cfg.Channels.WhatsApp.Enabled {
		enabled = append(enabled, "whatsapp")
	}
	if cfg.Channels.Telegram.Enabled {
		enabled = append(enabled, "telegram")
	}
	if cfg.Channels.Feishu.Enabled {
		enabled = append(enabled, "feishu")
	}
	if cfg.Channels.Discord.Enabled {
		enabled = append(enabled, "discord")
	}
	if cfg.Channels.MaixCam.Enabled {
		enabled = append(enabled, "maixcam")
	}
	if cfg.Channels.QQ.Enabled {
		enabled = append(enabled, "qq")
	}
	if cfg.Channels.DingTalk.Enabled {
		enabled = append(enabled, "dingtalk")
	}
	if cfg.Channels.Slack.Enabled {
		enabled = append(enabled, "slack")
	}
	if cfg.Channels.Matrix.Enabled {
		enabled = append(enabled, "matrix")
	}
	if cfg.Channels.LINE.Enabled {
		enabled = append(enabled, "line")
	}
	if cfg.Channels.OneBot.Enabled {
		enabled = append(enabled, "onebot")
	}
	if cfg.Channels.WeCom.Enabled {
		enabled = append(enabled, "wecom")
	}
	if cfg.Channels.Weixin.Enabled {
		enabled = append(enabled, "weixin")
	}
	if cfg.Channels.Pico.Enabled {
		enabled = append(enabled, "pico")
	}
	if cfg.Channels.PicoClient.Enabled {
		enabled = append(enabled, "pico_client")
	}
	if cfg.Channels.IRC.Enabled {
		enabled = append(enabled, "irc")
	}
	return enabled
}

func (r *AgentRegistry) workspaceForAgentIDLocked(agentID string) string {
	agent, ok := r.agents[routing.NormalizeAgentID(agentID)]
	if !ok || agent == nil {
		return ""
	}
	return agent.Workspace
}

func (r *AgentRegistry) defaultAgentIDLocked() string {
	if _, ok := r.agents[routing.DefaultAgentID]; ok {
		return routing.DefaultAgentID
	}
	if r.cfg != nil && len(r.cfg.Agents.List) > 0 {
		for _, agentCfg := range r.cfg.Agents.List {
			if !agentCfg.Default {
				continue
			}
			id := routing.NormalizeAgentID(agentCfg.ID)
			if _, ok := r.agents[id]; ok {
				return id
			}
		}
		id := routing.NormalizeAgentID(r.cfg.Agents.List[0].ID)
		if _, ok := r.agents[id]; ok {
			return id
		}
	}
	for id := range r.agents {
		return id
	}
	return ""
}

func cleanWorkspacePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return filepath.Clean(path)
}

func formatAgentDiscoverySection(currentAgentID string, agents []AgentDescriptor) string {
	if len(agents) <= 1 {
		return ""
	}

	payload := struct {
		CurrentAgentID string            `json:"current_agent_id"`
		Agents         []AgentDescriptor `json:"agents"`
	}{
		CurrentAgentID: strings.TrimSpace(currentAgentID),
		Agents:         agents,
	}

	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ""
	}

	var header strings.Builder
	header.WriteString("# Agent Discovery\n\n")
	if payload.CurrentAgentID != "" {
		fmt.Fprintf(
			&header,
			"You are agent %q. This registry is authoritative for the current PicoClaw instance and includes your own entry.\n",
			payload.CurrentAgentID,
		)
	} else {
		header.WriteString("This registry is authoritative for the current PicoClaw instance.\n")
	}
	header.WriteString(
		"Delegate based on available_tools first, then model, channels, and description. Use only agent IDs listed here.\n\n",
	)
	header.WriteString("```json\n")
	header.Write(encoded)
	header.WriteString("\n```")

	return header.String()
}
