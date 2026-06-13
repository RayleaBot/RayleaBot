package localaction

import "context"

func (s *Service) executeGovernanceCommandPolicyRead(ctx context.Context, pluginID string) (map[string]any, error) {
	if err := s.requireGovernanceCapability(ctx, pluginID, "governance.command_policy.read"); err != nil {
		return nil, err
	}

	response, err := s.governance.ReadCommandPolicy(ctx)
	if err != nil {
		return nil, mapGovernanceRuntimeError("governance.command_policy.read failed", err)
	}
	return map[string]any{
		"default_level": response.DefaultLevel,
		"cooldown":      response.Cooldown,
		"commands":      response.Commands,
	}, nil
}
