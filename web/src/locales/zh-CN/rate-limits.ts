export const rateLimits = {
  title: '限流中心',
  sections: {
    settings: '限流配置',
    userCommand: '用户命令',
    groupCommand: '群命令',
    cooldownReply: '冷却提示',
    pluginMessage: '插件消息',
    targetMessage: '目标消息',
  },
  fields: {
    cooldownReply: '命中后发送冷却提示',
  },
  summary: {
    userCommand: '用户命令',
    groupCommand: '群命令',
    pluginMessage: '插件消息',
    targetMessage: '目标消息',
    userCommandMeta: '按同一用户累计命令触发次数。',
    groupCommandMeta: '按同一群累计命令触发次数。',
    pluginMessageMeta: '按同一插件累计外发消息次数。',
    targetMessageMeta: '按同一群或同一私聊目标累计外发消息次数。',
  },
  hints: {
    userCommandRateLimit: '同一用户在一个滑动时间窗口内最多触发多少次命令。命中后拒绝本次命令；开启冷却提示时会尝试发送提示消息，提示消息仍受目标消息限流约束。',
    groupCommandRateLimit: '同一群在一个滑动时间窗口内合计最多触发多少次命令。命中后拒绝本次命令；开启冷却提示时会尝试发送提示消息，提示消息仍受目标消息限流约束。',
    cooldownReply: '用户命令或群命令命中限流时发送提示消息；提示消息按目标消息限流排队，超过等待上限或上下文取消时放弃发送。',
    pluginMessageRateLimit: '单个插件在一个滑动时间窗口内最多发出多少条消息。命中后进入 FIFO 排队等待，超过消息熔断时长或上下文取消时放弃发送并记录限流结果。',
    targetMessageRateLimit: '同一群或同一私聊目标在一个滑动时间窗口内最多接收多少条消息。命中后进入 FIFO 排队等待，超过消息熔断时长或上下文取消时放弃发送并记录限流结果。',
  },
  status: {
    unsaved: '有未保存更改',
    savedHot: '保存完成，已生效',
    savedRestart: '保存完成，重启后生效',
  },
} as const
