package defaultmodules

import (
	"context"

	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/actions/governanceaction"
)

func init() {
	registerGovernance("governance.blacklist.read", "plugin-protocol.action_governance_blacklist_read", governanceaction.BlacklistRead)
	registerGovernance("governance.blacklist.write", "plugin-protocol.action_governance_blacklist_write", governanceaction.BlacklistWrite)
	registerGovernance("governance.whitelist.read", "plugin-protocol.action_governance_whitelist_read", governanceaction.WhitelistRead)
	registerGovernance("governance.whitelist.write", "plugin-protocol.action_governance_whitelist_write", governanceaction.WhitelistWrite)
	registerGovernance("governance.command_policy.read", "plugin-protocol.action_governance_command_policy_read", governanceaction.CommandPolicyRead)
}

func registerGovernance(action string, schema string, execute func(context.Context, actions.Deps, actions.ActionRequest) (map[string]any, error)) {
	register(Metadata{
		Action:         action,
		Capability:     action,
		RequestSchema:  schema,
		ResponseSchema: "plugin-protocol.local_action_result",
		AuditFields:    []string{"plugin_id", "operation", "entry_type", "target_id"},
		ErrorCodes:     commonErrorCodes("platform.resource_missing"),
	}, func(deps actions.Deps) actions.ActionHandler {
		return func(ctx context.Context, req actions.ActionRequest) (map[string]any, error) {
			return execute(ctx, deps, req)
		}
	})
}
