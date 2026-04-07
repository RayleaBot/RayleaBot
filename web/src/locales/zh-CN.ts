import { app, routes, shell } from './zh-CN/app'
import { dashboard } from './zh-CN/dashboard'
import { plugins } from './zh-CN/plugins'
import { tasks } from './zh-CN/tasks'
import { logs } from './zh-CN/logs'
import { config } from './zh-CN/config'
import { auth } from './zh-CN/auth'
import { display } from './zh-CN/display'
import { errors } from './zh-CN/errors'

export const zhCN = {
  app,
  routes,
  shell,
  dashboard,
  plugins,
  tasks,
  logs,
  config,
  auth,
  display,
  errors,
} as const
