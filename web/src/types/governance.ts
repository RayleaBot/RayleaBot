import type { components } from './generated'

export type GovernanceScope = components['schemas']['GovernanceScope']
export type BlacklistEntry = components['schemas']['BlacklistEntry']
export type GovernanceEntryType = components['schemas']['GovernanceEntryType']
export type GovernanceEntryUpsertRequest = components['schemas']['GovernanceEntryUpsertRequest']
export type CommandPermissionLevel = components['schemas']['CommandPermissionLevel']
export type CommandPermissionSource = components['schemas']['CommandPermissionSource']
export type GovernanceBlacklistResponse = components['schemas']['GovernanceBlacklistResponse']
export type GovernanceWhitelistResponse = components['schemas']['GovernanceWhitelistResponse']

type GovernanceCommandPolicyEntryContract = components['schemas']['GovernanceCommandPolicyEntry']
type GovernanceCommandPolicyResponseContract = components['schemas']['GovernanceCommandPolicyResponse']

export interface GovernanceCommandPolicyEntry extends Omit<GovernanceCommandPolicyEntryContract, 'declared_permission'> {
  declared_permission: CommandPermissionLevel | null
}

export interface GovernanceCommandPolicyResponse extends Omit<GovernanceCommandPolicyResponseContract, 'commands'> {
  commands: GovernanceCommandPolicyEntry[]
}
