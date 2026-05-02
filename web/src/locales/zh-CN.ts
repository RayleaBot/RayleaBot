import { app, fallback, routes, shell } from './zh-CN/app'
import { dashboard } from './zh-CN/dashboard'
import { commands } from './zh-CN/commands'
import { permissionPolicy } from './zh-CN/permission-policy'
import { rateLimits } from './zh-CN/rate-limits'
import { accessLists } from './zh-CN/access-lists'
import { plugins } from './zh-CN/plugins'
import { tasks } from './zh-CN/tasks'
import { logs } from './zh-CN/logs'
import { protocols } from './zh-CN/protocols'
import { config } from './zh-CN/config'
import { renderTemplates } from './zh-CN/render-templates'
import { auth } from './zh-CN/auth'
import { display } from './zh-CN/display'
import { errors } from './zh-CN/errors'

export const zhCN = {
  app,
  fallback,
  routes,
  shell,
  dashboard,
  commands,
  permissionPolicy,
  rateLimits,
  accessLists,
  plugins,
  tasks,
  logs,
  protocols,
  config,
  renderTemplates,
  auth,
  display,
  errors,
} as const
