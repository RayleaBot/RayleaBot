import path from 'node:path'
import { readFile } from 'node:fs/promises'
import YAML from 'yaml'

const repoRoot = path.resolve(import.meta.dirname, '../../..')

async function readFixture(relativePath) {
  const absolutePath = path.join(repoRoot, relativePath)
  const raw = await readFile(absolutePath, 'utf8')
  if (relativePath.endsWith('.json')) {
    return JSON.parse(raw)
  }
  return YAML.parse(raw)
}

export const errorCodes = (await readFixture('contracts/error-codes.yaml')).codes

export const fixtures = {
  healthz: await readFixture('fixtures/web-api/ok.healthz-response.yaml'),
  readyz: await readFixture('fixtures/web-api/edge.readyz-degraded-response.yaml'),
  setupAdmin: await readFixture('fixtures/web-api/ok.setup-admin.yaml'),
  setupAdminDenied: await readFixture('fixtures/web-api/edge.setup-admin-already-initialized.yaml'),
  setupStatus: await readFixture('fixtures/web-api/ok.setup-status.yaml'),
  sessionLogin: await readFixture('fixtures/web-api/ok.session-login.yaml'),
  sessionDenied: await readFixture('fixtures/web-api/invalid.session-login-bad-credentials.yaml'),
  configGet: await readFixture('fixtures/web-api/ok.config-get-response.yaml'),
  protocolSnapshot: await readFixture('fixtures/web-api/ok.protocol-onebot11-snapshot.yaml'),
  protocolCompatibility: await readFixture('fixtures/web-api/ok.protocol-onebot11-compatibility.yaml'),
  logsList: await readFixture('fixtures/web-api/ok.logs-list-response.yaml'),
  logDetail: await readFixture('fixtures/web-api/ok.log-detail-response.yaml'),
  logDetailNotFound: await readFixture('fixtures/web-api/edge.log-detail-not-found.yaml'),
  systemStatus: await readFixture('fixtures/web-api/ok.system-status.yaml'),
  systemDiagnostics: await readFixture('fixtures/web-api/ok.system-diagnostics.yaml'),
  updateStatus: await readFixture('fixtures/web-api/ok.update-status.yaml'),
  updateCheck: await readFixture('fixtures/web-api/ok.update-check.yaml'),
  systemShutdown: await readFixture('fixtures/web-api/ok.system-shutdown.yaml'),
  systemBackupAccepted: await readFixture('fixtures/web-api/ok.system-backup-accepted.yaml'),
  renderTemplatesList: await readFixture('fixtures/web-api/ok.system-render-templates-list-response.yaml'),
  renderTemplateDetail: await readFixture('fixtures/web-api/ok.system-render-template-detail-response.yaml'),
  renderTemplateNotFound: await readFixture('fixtures/web-api/invalid.system-render-template-not-found.yaml'),
  schedulerJobsList: await readFixture('fixtures/web-api/ok.system-scheduler-jobs-list.yaml'),
  schedulerJobTriggered: await readFixture('fixtures/web-api/ok.system-scheduler-job-triggered.yaml'),
  systemDiagnosticsExport: await readFixture('fixtures/web-api/ok.system-diagnostics-export.yaml'),
  pluginEnable: await readFixture('fixtures/web-api/ok.plugins-enable-response.yaml'),
  pluginDisable: await readFixture('fixtures/web-api/edge.plugins-disable-response.yaml'),
  pluginReload: await readFixture('fixtures/web-api/ok.plugins-reload-response.yaml'),
  pluginInstallAccepted: await readFixture('fixtures/web-api/ok.plugins-install-accepted.yaml'),
  pluginList: await readFixture('fixtures/web-api/ok.plugins-list-response.yaml'),
  pluginStoreList: await readFixture('fixtures/web-api/ok.plugin-store-list.yaml'),
  pluginStoreSources: await readFixture('fixtures/web-api/ok.plugin-store-sources.yaml'),
  pluginStoreInspection: await readFixture('fixtures/web-api/ok.plugin-store-inspection.yaml'),
  pluginStoreSourceRefresh: await readFixture('fixtures/web-api/ok.plugin-store-source-refresh.yaml'),
  pluginDetail: await readFixture('fixtures/web-api/ok.plugin-detail-response.yaml'),
  pluginDetailManagementUI: await readFixture('fixtures/web-api/ok.plugin-detail-response.management-ui.yaml'),
  pluginSettings: await readFixture('fixtures/web-api/ok.plugin-settings-response.yaml'),
  pluginUninstallAccepted: await readFixture('fixtures/web-api/ok.plugins-uninstall-accepted.yaml'),
  governanceBlacklist: await readFixture('fixtures/web-api/ok.governance-blacklist-response.yaml'),
  governanceWhitelist: await readFixture('fixtures/web-api/ok.governance-whitelist-response.yaml'),
  governanceCommandPolicy: await readFixture('fixtures/web-api/ok.governance-command-policy-response.yaml'),
  wsEvents: await readFixture('fixtures/websocket/edge.events-received-degraded.json'),
  wsEventsProtocolSnapshot: await readFixture('fixtures/websocket/ok.events-received-protocol-snapshot.json'),
  wsConsole: await readFixture('fixtures/websocket/ok.plugins-console-stderr.json'),
  wsSessionExpired: await readFixture('fixtures/websocket/edge.session-expired.json'),
}
