export const rateLimits = {
  title: '限流中心',
  subtitle: '限制用户和群触发命令的频率，以及发往同一目标的消息速率。',
  sections: {
    userCommand: '用户命令',
    groupCommand: '群命令',
    cooldownReply: '冷却提示',
    targetMessage: '目标消息',
  },
  fields: {
    cooldownReply: '命中后发送冷却提示',
  },
  hints: {
    userCommandRateLimit: '同一用户在一个滑动时间窗口内最多触发多少次命令。命中后拒绝本次命令；开启冷却提示时会尝试发送提示消息，提示消息仍受目标消息限流约束。',
    groupCommandRateLimit: '同一群在一个滑动时间窗口内合计最多触发多少次命令。命中后拒绝本次命令；开启冷却提示时会尝试发送提示消息，提示消息仍受目标消息限流约束。',
    cooldownReply: '用户命令或群命令命中限流时发送提示消息；提示消息按目标消息限流排队，超过等待上限或上下文取消时放弃发送。',
    targetMessageRateLimit: '同一群或同一私聊目标在一个滑动时间窗口内最多接收多少条消息。命中后进入 FIFO 排队等待，请求取消或等待超时时放弃发送并记录限流结果。',
  },
} as const
