import type { components } from './generated'

export type BlacklistEntry = components['schemas']['BlacklistEntry']
export type GovernanceEntryType = components['schemas']['GovernanceEntryType']
export type GovernanceEntryUpsertRequest = components['schemas']['GovernanceEntryUpsertRequest']
export type GovernanceEntryUpsertResponse = components['schemas']['GovernanceEntryUpsertResponse']
export type CommandPermissionLevel = components['schemas']['CommandPermissionLevel']
export type CommandPermissionSource = components['schemas']['CommandPermissionSource']
export type GovernanceCommandCooldown = components['schemas']['GovernanceCommandCooldown']
export type GovernanceBlacklistResponse = components['schemas']['GovernanceBlacklistResponse']
export type GovernanceWhitelistResponse = components['schemas']['GovernanceWhitelistResponse']
export type GovernanceWhitelistStateResponse = components['schemas']['GovernanceWhitelistStateResponse']
export type GovernanceWhitelistStateUpdateRequest = components['schemas']['GovernanceWhitelistStateUpdateRequest']

type GovernanceCommandPolicyEntryContract = components['schemas']['GovernanceCommandPolicyEntry']
type GovernanceCommandPolicyResponseContract = components['schemas']['GovernanceCommandPolicyResponse']

export interface GovernanceCommandPolicyEntry extends Omit<GovernanceCommandPolicyEntryContract, 'declared_permission'> {
  declared_permission: CommandPermissionLevel | null
}

export interface GovernanceCommandPolicyResponse extends Omit<GovernanceCommandPolicyResponseContract, 'commands'> {
  commands: GovernanceCommandPolicyEntry[]
}
