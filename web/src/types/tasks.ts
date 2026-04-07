export type TaskStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'interrupted'

export type TaskType =
  | 'plugin.install'
  | 'plugin.uninstall'
  | 'plugin.reload'
  | 'backup.create'
  | 'recovery.recheck'
  | 'recovery.confirm'
  | 'restore.apply'
  | 'config.migrate'
  | 'db.migrate'
  | 'runtime.bootstrap'
  | 'render.preview'

export interface TaskResultSummary {
  summary: string
  details?: Record<string, unknown>
}

export interface TaskErrorSummary {
  code: string
  message: string
  details?: Record<string, unknown>
}

export interface TaskSummary {
  task_id: string
  task_type: TaskType
  status: TaskStatus
  progress?: number
  summary: string
  started_at?: string
  finished_at?: string
  result?: TaskResultSummary
  error?: TaskErrorSummary
}

export interface TaskListResponse {
  items: TaskSummary[]
}

export interface TaskDetailResponse {
  task: TaskSummary
}

export interface TaskAcceptedResponse {
  task_id: string
}
