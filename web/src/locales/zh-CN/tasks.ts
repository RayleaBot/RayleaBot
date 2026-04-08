export const tasks = {
  title: '任务',
  refresh: '刷新任务',
  detailTitle: '任务详情',
  fields: {
    id: '任务 ID',
    type: '任务类型',
    status: '任务状态',
    progress: '任务进度',
    summary: '摘要',
    started: '开始时间',
    finished: '结束时间',
    result: '执行结果',
    error: '错误信息',
  },
  actions: {
    detail: '查看详情',
    cancel: '请求取消',
  },
  previewAlt: '图片预览结果',
  recoverySummary: '恢复摘要',
  cancelAccepted: '取消请求已发送',
} as const
