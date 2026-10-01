export const rateLimits = {
  title: '限流中心',
  subtitle: '限制用户和群触发指令的频率，以及发往同一目标的消息速率。',
  sections: {
    userCommand: '用户指令',
    groupCommand: '群指令',
    cooldownReply: '冷却提示',
    targetMessage: '外发消息',
  },
  fields: {
    cooldownReply: '命中后发送冷却提示',
  },
  hints: {
    userCommandRateLimit: '同一用户在一个滑动时间窗口内最多触发多少次指令。命中后拒绝本次指令；开启冷却提示时会尝试发送提示消息，提示消息同样受外发消息限速。',
    groupCommandRateLimit: '同一群在一个滑动时间窗口内合计最多触发多少次指令。命中后拒绝本次指令；开启冷却提示时会尝试发送提示消息，提示消息同样受外发消息限速。',
    cooldownReply: '用户指令或群指令命中限流时发送提示消息；提示消息同样受外发消息限速，排队超时或请求被取消时放弃发送。',
    targetMessageRateLimit: '机器人在一个滑动时间窗口内最多向同一个群或私聊发送多少条消息。超出的消息按先后顺序排队，等待超时或请求被取消时放弃发送，并记录限流结果。',
  },
} as const
