package agent

import (
	"sort"
	"strings"

	"github.com/sipeed/picoclaw/pkg/config"
)

func resolveAgentToolAllowlist(agentCfg *config.AgentConfig) []string {
	if agentCfg == nil || agentCfg.Tools == nil {
		return nil
	}

	allowlist := make(map[string]struct{}, len(agentCfg.Tools))
	for _, raw := range agentCfg.Tools {
		trimmed := strings.ToLower(strings.TrimSpace(raw))
		if trimmed == "" {
			continue
		}
		allowlist[trimmed] = struct{}{}
	}

	result := make([]string, 0, len(allowlist))
	for name := range allowlist {
		result = append(result, name)
	}
	sort.Strings(result)
	return result
}
