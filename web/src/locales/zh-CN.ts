import { app, routes, shell } from './zh-CN/app'
import { dashboard } from './zh-CN/dashboard'
import { commands } from './zh-CN/commands'
import { permissionPolicy } from './zh-CN/permission-policy'
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
  routes,
  shell,
  dashboard,
  commands,
  permissionPolicy,
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
