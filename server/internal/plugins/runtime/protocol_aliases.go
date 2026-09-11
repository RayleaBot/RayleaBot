package runtime

import (
	"github.com/RayleaBot/RayleaBot/server/internal/bot/chatevent"
	"github.com/RayleaBot/RayleaBot/server/internal/plugins/pluginwire"
)

// Protocol representations are generated identically for the host and the standalone SDK.
type FrameEnvelope = pluginwire.FrameEnvelope
type InitFrame = pluginwire.InitFrame
type InitProgressFrame = pluginwire.InitProgressFrame
type InitAckFrame = pluginwire.InitAckFrame
type EventFrame = pluginwire.EventFrame
type ProtocolEventFrame = pluginwire.ProtocolEventFrame
type ProtocolActorFrame = pluginwire.ProtocolActorFrame
type ProtocolTargetFrame = pluginwire.ProtocolTargetFrame
type ProtocolMessageFrame = pluginwire.ProtocolMessageFrame
type ProtocolWebhookFrame = pluginwire.ProtocolWebhookFrame
type ProtocolPayloadFrame = pluginwire.ProtocolPayloadFrame
type ProtocolOneBotPayloadFrame = pluginwire.ProtocolOneBotPayloadFrame
type ProtocolOneBotSenderFrame = pluginwire.ProtocolOneBotSenderFrame
type ProtocolQQOfficialPayloadFrame = pluginwire.ProtocolQQOfficialPayloadFrame
type ProtocolSegmentFrame = pluginwire.ProtocolSegmentFrame
type ProtocolOutboundMessageFrame = pluginwire.ProtocolOutboundMessageFrame
type PingFrame = pluginwire.PingFrame
type ShutdownFrame = pluginwire.ShutdownFrame
type ErrorFrame = pluginwire.ErrorFrame
type ResultFrame = pluginwire.ResultFrame
type ActionFrame = pluginwire.ActionFrame
type ProtocolActionMessageSendFrame = pluginwire.ProtocolActionMessageSendFrame
type ProtocolActionLoggerWriteFrame = pluginwire.ProtocolActionLoggerWriteFrame
type ProtocolActionStorageKVFrame = pluginwire.ProtocolActionStorageKVFrame
type ProtocolActionStorageFileFrame = pluginwire.ProtocolActionStorageFileFrame
type ProtocolActionHTTPRequestFrame = pluginwire.ProtocolActionHTTPRequestFrame
type ProtocolActionPluginListFrame = pluginwire.ProtocolActionPluginListFrame
type ProtocolActionSecretReadFrame = pluginwire.ProtocolActionSecretReadFrame
type ProtocolActionThirdPartyAccountReadFrame = pluginwire.ProtocolActionThirdPartyAccountReadFrame
type ProtocolActionThirdPartyAccountValidateFrame = pluginwire.ProtocolActionThirdPartyAccountValidateFrame
type ProtocolActionThirdPartyResolveFrame = pluginwire.ProtocolActionThirdPartyResolveFrame
type ProtocolActionConfigWriteFrame = pluginwire.ProtocolActionConfigWriteFrame
type ProtocolActionGovernanceBlacklistWriteFrame = pluginwire.ProtocolActionGovernanceBlacklistWriteFrame
type ProtocolActionGovernanceWhitelistWriteFrame = pluginwire.ProtocolActionGovernanceWhitelistWriteFrame
type ProtocolActionSchedulerCreateFrame = pluginwire.ProtocolActionSchedulerCreateFrame
type ProtocolActionRenderImageFrame = pluginwire.ProtocolActionRenderImageFrame
type ProtocolRenderImageResourceFrame = pluginwire.ProtocolRenderImageResourceFrame

func wireBotIdentities(bots []chatevent.BotIdentity) []pluginwire.BotIdentity {
	result := make([]pluginwire.BotIdentity, 0, len(bots))
	for _, bot := range bots {
		result = append(result, pluginwire.BotIdentity(bot))
	}
	return result
}
