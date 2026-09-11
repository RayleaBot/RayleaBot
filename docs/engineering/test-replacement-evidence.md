# Services 测试替代证据

这份记录对应执行计划 P20 的 services 装配替代。原 `server/tests/services/` 的 80 个测试逐项保留名称和业务断言，移到相应领域的外部测试包；构造对象改用正式 `Deps`，不再复制 App、Platform、EventStack 或 Services 状态。

补充评审后的测试边界：管理与安装测试直接使用真实 `catalog.Catalog`，删除两份复制排序、状态冲突和刷新逻辑的 `testCatalog`。目录并发测试验证更新结果、稳定成员、读快照与最终状态，并纳入 race 检查。错误与事件选择使用 code、message_key、outcome、event_id 等结构字段；中文名称、用户输入、菜单语法、脱敏结果及专门的格式化测试继续保留数据断言。QQ 官方网关和媒体回复改为显式选择的 [人工 Smoke](./manual-smoke.md)，不计入默认自动覆盖。

`harness_test.go`、无语义转发器及空的 services 测试目录已移除。共享渲染 runner 和取消感知的事件记录运行时位于 `tests/testutil`，各测试创建并关闭自己的 SQLite、Dispatcher 和 HTTP server。日志等待使用订阅与超时预算。

## 装配证据

`server/tests/integration/service_composition_test.go` 使用真实 `app.New`、临时 SQLite 和原生 Go SDK 插件进程，验证 HTTP 设置写入、命令注册更新、`config.changed` 通知、插件 `config.write` 回写、配置前缀热更新、治理黑名单与关闭状态。生产 App 不提供新的运行期 setter。

## 原用例映射

旧路径均相对于已删除的 `server/tests/services/`。以下每项决定均为“移动并保留行为”；新证据保持原测试名。

| 旧文件与测试 | 风险 | 新证据 |
| --- | --- | --- |
| `app_run_chat_policy_builtin_menu_test.go` · `TestHandleAdapterEventUsesIndependentBuiltinMenuPrefix` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestApplyChatPolicyDoesNotTreatPluginCommandAsBuiltinWhenMenuPrefixDiffers` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestHandleAdapterEventRendersBuiltinMenuPluginPrefixesAsHeaderBadge` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestHandleAdapterEventMatchesBuiltinPluginSuffixHelp` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestHandleAdapterEventSkipsMissingBuiltinPluginMenuTarget` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestHandleAdapterEventDoesNotTreatExactPluginCommandAsBuiltinSuffixMenu` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestHandleAdapterEventBlocksBuiltinMenuWhenBlacklistApplies` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestHandleAdapterEventBlocksBuiltinMenuWhenCooldownApplies` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_builtin_menu_test.go` · `TestApplyChatPolicyLogsCooldownReplySuccess` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_builtin_menu_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestApplyChatPolicyAppliesTargetLimitToCooldownReply` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestApplyChatPolicyCancelsCooldownReplyTargetLimit` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestApplyChatPolicyUsesCanonicalUserCooldownForPrivateCommand` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestApplyChatPolicyUsesCanonicalUserCooldownForGroupCommand` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestApplyChatPolicyUsesCanonicalGroupCooldown` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestApplyChatPolicyUsesCanonicalCooldownReplyFlag` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestApplyChatPolicyUsesCanonicalPermissionAndSuperAdmin` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_cooldown_test.go` · `TestHandleAdapterEventSendsBuiltinMenuImageWithoutPluginDispatch` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_cooldown_test.go) |
| `app_run_chat_policy_logging_test.go` · `TestApplyChatPolicyLogsCooldownReplyFailure` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_logging_test.go) |
| `app_run_chat_policy_logging_test.go` · `TestApplyHotReloadableFieldsReloadsCommandPolicy` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/config/runtime/chatpolicy_reload_test.go) |
| `app_run_chat_policy_test.go` · `TestCommandInfoForEventUsesDefaultLevelForOmittedPermission` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestResolveChatPolicyConfigUsesConfiguredFields` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventBlocksBlacklistedMessageBeforeBridge` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventKeepsBlacklistedNonCommandMessageSilent` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventBlocksCommandWhenNotWhitelistedBeforeBridge` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventLogsWhitelistedCommandRejection` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventLogsBlacklistedCommandRejection` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventUsesMostStrictMatchingCommandPermission` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventLogsPermissionDeniedCommandRejection` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestHandleAdapterEventLogsConflictingCommandRejectionWithoutPluginID` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_chat_policy_test.go` · `TestApplyChatPolicySendsCooldownReplyForGroupCommand` | 命令权限、冷却、黑白名单或内置菜单分流 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_behavior_test.go) |
| `app_run_config_http_test.go` · `TestApplyHotReloadableFieldsClassifiesCanonicalPaths` | 配置应用策略与真实协作者热更新 | [同名测试](../../server/internal/config/runtime/http_effects_test.go) |
| `app_run_config_http_test.go` · `TestApplyHotReloadableFieldsFallsBackToRestartRequiredWhenAdapterReloadFails` | 配置应用策略与真实协作者热更新 | [同名测试](../../server/internal/config/runtime/http_effects_test.go) |
| `app_run_config_http_test.go` · `TestApplyHotReloadableFieldsClassifiesRenderDefaultsAsAppliedNow` | 配置应用策略与真实协作者热更新 | [同名测试](../../server/internal/config/runtime/http_effects_test.go) |
| `app_run_config_http_test.go` · `TestHandleConfigPutHotReloadsRenderDefaults` | 配置应用策略与真实协作者热更新 | [同名测试](../../server/internal/config/runtime/http_effects_test.go) |
| `app_run_config_http_test.go` · `TestHandleConfigPutHotReloadsOutboundLimiterMessageFields` | 配置应用策略与真实协作者热更新 | [同名测试](../../server/internal/config/runtime/http_effects_test.go) |
| `app_run_event_ingress_metadata_test.go` · `TestEventIngressEnrichesMetadataBeforeBridgeDispatch` | 元数据在投递前补齐 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_metadata_test.go) |
| `app_run_plugin_local_actions_file_test.go` · `TestExecuteStorageFileRoundTripUsesImplicitPrivateNamespace` | 插件私有文件命名空间与内容往返 | [同名测试](../../server/internal/plugins/actions/service_integration_file_test.go) |
| `app_run_plugin_local_actions_file_test.go` · `TestExecuteStorageFileNamespacesPlugins` | 插件私有文件命名空间与内容往返 | [同名测试](../../server/internal/plugins/actions/service_integration_file_test.go) |
| `app_run_plugin_local_actions_http_test.go` · `TestExecuteHTTPRequestUsesPermissionedScopeAndReturnsText` | 网络权限边界 | [同名测试](../../server/internal/plugins/actions/service_integration_http_test.go) |
| `app_run_plugin_local_actions_http_test.go` · `TestExecuteHTTPRequestRejectsPrivateHost` | 网络权限边界 | [同名测试](../../server/internal/plugins/actions/service_integration_http_test.go) |
| `app_run_plugin_local_actions_logging_storage_test.go` · `TestExecuteLoggerWriteAppliesRateLimit` | 本地动作、存储、日志限流、设置通知或治理事件 | [同名测试](../../server/internal/plugins/actions/service_integration_logging_storage_test.go) |
| `app_run_plugin_local_actions_logging_storage_test.go` · `TestExecuteStorageKVRoundTrip` | 本地动作、存储、日志限流、设置通知或治理事件 | [同名测试](../../server/internal/plugins/actions/service_integration_logging_storage_test.go) |
| `app_run_plugin_local_actions_logging_storage_test.go` · `TestExecuteConfigWriteDispatchesConfigChanged` | 本地动作、存储、日志限流、设置通知或治理事件 | [同名测试](../../server/internal/plugins/actions/service_integration_logging_storage_test.go) |
| `app_run_plugin_local_actions_logging_storage_test.go` · `TestExecuteGovernanceActionsRejectMissingPermission` | 本地动作、存储、日志限流、设置通知或治理事件 | [同名测试](../../server/internal/plugins/actions/service_integration_logging_storage_test.go) |
| `app_run_plugin_local_actions_logging_storage_test.go` · `TestExecuteGovernanceActionsRoundTrip` | 本地动作、存储、日志限流、设置通知或治理事件 | [同名测试](../../server/internal/plugins/actions/service_integration_logging_storage_test.go) |
| `app_run_plugin_local_actions_logging_storage_test.go` · `TestExecuteGovernanceWritePublishesGovernanceChanged` | 本地动作、存储、日志限流、设置通知或治理事件 | [同名测试](../../server/internal/plugins/actions/service_integration_logging_storage_test.go) |
| `app_run_plugin_local_actions_logging_storage_test.go` · `TestExecuteSchedulerCreateUpsertDoesNotWriteManagementLog` | 本地动作、存储、日志限流、设置通知或治理事件 | [同名测试](../../server/internal/plugins/actions/service_integration_logging_storage_test.go) |
| `app_run_plugin_local_actions_onebot_test.go` · `TestExecuteOneBotLocalActionMessageHistoryGet` | 协议权限、provider门禁与断连 | [同名测试](../../server/internal/plugins/actions/service_integration_onebot_test.go) |
| `app_run_plugin_local_actions_onebot_test.go` · `TestExecuteOneBotLocalActionProviderMismatch` | 协议权限、provider门禁与断连 | [同名测试](../../server/internal/plugins/actions/service_integration_onebot_test.go) |
| `app_run_plugin_local_actions_onebot_test.go` · `TestExecuteOneBotLocalActionProviderExtensionUsesDetectedProvider` | 协议权限、provider门禁与断连 | [同名测试](../../server/internal/plugins/actions/service_integration_onebot_test.go) |
| `app_run_plugin_local_actions_onebot_test.go` · `TestExecuteOneBotLocalActionRejectsMissingPermission` | 协议权限、provider门禁与断连 | [同名测试](../../server/internal/plugins/actions/service_integration_onebot_test.go) |
| `app_run_plugin_local_actions_onebot_test.go` · `TestExecuteOneBotLocalActionConnectionLossKeepsPluginRunning` | 协议权限、provider门禁与断连 | [同名测试](../../server/internal/plugins/actions/service_integration_onebot_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageReturnsArtifact` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageInjectsPluginFooter` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageResolvesOwnPluginTemplateShortID` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageRejectsOtherPluginTemplate` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageRejectsUnknownOtherPluginTemplate` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageInjectsGroupIdentityFromParentEvent` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageInjectsPrivateIdentityWithoutGroup` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageKeepsPrivateSuperAdminBadge` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageAppliesIdentityBadgeRulesToStatusPanel` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_render_test.go` · `TestExecuteRenderImageLeavesNonIdentityTemplateDataUnchanged` | 渲染产物、模板所有权与身份信息 | [同名测试](../../server/internal/plugins/actions/service_integration_render_test.go) |
| `app_run_plugin_local_actions_test.go` · `TestExecutePluginPrivateKVWithoutDeclaredPermission` | 私有存储、插件可见性与凭据隔离 | [同名测试](../../server/internal/plugins/actions/service_integration_test.go) |
| `app_run_plugin_local_actions_test.go` · `TestExecutePluginListUsesDeclaredPermission` | 私有存储、插件可见性与凭据隔离 | [同名测试](../../server/internal/plugins/actions/service_integration_test.go) |
| `app_run_plugin_local_actions_test.go` · `TestExecutePluginListCallerVisibilityFiltersCommands` | 私有存储、插件可见性与凭据隔离 | [同名测试](../../server/internal/plugins/actions/service_integration_test.go) |
| `app_run_plugin_local_actions_test.go` · `TestExecutePluginListCallerVisibilityFiltersHelp` | 私有存储、插件可见性与凭据隔离 | [同名测试](../../server/internal/plugins/actions/service_integration_test.go) |
| `app_run_plugin_local_actions_test.go` · `TestExecuteSecretReadReturnsPluginScopedValue` | 私有存储、插件可见性与凭据隔离 | [同名测试](../../server/internal/plugins/actions/service_integration_test.go) |
| `app_run_plugin_local_actions_test.go` · `TestExecuteSecretReadRejectsInvalidKey` | 私有存储、插件可见性与凭据隔离 | [同名测试](../../server/internal/plugins/actions/service_integration_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestHandlePluginManagementUIStaticServesScopedAssets` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestPluginUIOriginRejectsAdminOrigin` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestHandlePluginManagementUIStaticRejectsParentEscape` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestHandlePluginSettingsGetMergesDefaultsAndPersistedValues` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestHandlePluginSettingsPutDispatchesConfigChanged` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestHandlePluginSecretsGetAndPutAreScopedToPlugin` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestHandlePluginSecretsPutRejectsInvalidKey` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_management_ui_http_test.go` · `TestHandlePluginSettingsRejectsInvalidPluginSnapshots` | 网络权限边界 | [同名测试](../../server/internal/management/plugin_management_ui_test.go) |
| `app_run_plugin_webhooks_test.go` · `TestHandlePluginWebhookUsesStaticManifestRegistration` | manifest注册、认证、事件投递与体积限制 | [同名测试](../../server/internal/plugins/webhook/manifest_http_test.go) |
| `app_run_plugin_webhooks_test.go` · `TestHandlePluginWebhookRejectsManifestBodyLimit` | manifest注册、认证、事件投递与体积限制 | [同名测试](../../server/internal/plugins/webhook/manifest_http_test.go) |
| `app_run_runtime_mainline_test.go` · `TestPluginDiscoveryContextUsesOnlyInstalledRoot` | 发现根与命令参数规范化 | [同名测试](../../server/internal/plugins/catalog/discovery_scope_test.go) |
| `app_run_runtime_mainline_test.go` · `TestEnrichCommandEventAddsCommandPayload` | 发现根与命令参数规范化 | [同名测试](../../server/internal/bot/pipeline/chatpolicy/ingress_command_payload_test.go) |

没有因文件长度或替身数量删除行为用例。测试总数映射检查要求旧名称 80 个、新位置各出现一次；运行验证仍单独检查各包及真实 App 回归的执行结果。
