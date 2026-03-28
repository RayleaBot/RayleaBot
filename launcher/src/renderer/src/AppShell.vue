<script setup lang="ts">
import { computed } from "vue";
import type { LauncherSettings, LauncherSnapshot, LauncherServiceState } from "@shared/launcher-models";

type SectionId = "status" | "environment" | "diagnostics" | "settings";

const props = withDefaults(
  defineProps<{
    snapshot: LauncherSnapshot;
    activeSection: SectionId;
    settingsDraft?: LauncherSettings;
    editingSettings?: boolean;
    diagnosticsSummary?: string;
    busyAction?: string | null;
    controlsDisabled?: boolean;
  }>(),
  {
    settingsDraft: undefined,
    editingSettings: false,
    diagnosticsSummary: "",
    busyAction: null,
    controlsDisabled: false,
  },
);

const emit = defineEmits<{
  navigate: [SectionId];
  refresh: [];
  start: [];
  stop: [];
  openWeb: [];
  openReleasePage: [];
  openLogs: [];
  beginEdit: [];
  cancelEdit: [];
  saveSettings: [];
  updateSettings: [LauncherSettings];
  chooseServer: [];
  chooseConfig: [];
  chooseWorkdir: [];
  exit: [];
}>();

const sections = [
  { id: "status", title: "状态" },
  { id: "environment", title: "环境检查" },
  { id: "diagnostics", title: "日志与诊断" },
  { id: "settings", title: "设置" },
] as const satisfies ReadonlyArray<{ id: SectionId; title: string }>;

const severityRank: Record<string, number> = { error: 0, warning: 1, ok: 2 };

const primaryIssue = computed(() => {
  return [...props.snapshot.environmentChecks].sort((left, right) => severityRank[left.severity] - severityRank[right.severity])[0];
});

const groupedChecks = computed(() => {
  return {
    blocking: props.snapshot.environmentChecks.filter((item) => item.severity === "error"),
    warnings: props.snapshot.environmentChecks.filter((item) => item.severity === "warning"),
    ready: props.snapshot.environmentChecks.filter((item) => item.severity === "ok"),
  };
});

const trayStatus = computed(() => statusSummary(props.snapshot.serviceState));

function statusSummary(state: LauncherServiceState) {
  switch (state) {
    case "stopped":
      return "未启动";
    case "starting":
      return "启动中";
    case "external_service":
    case "ready":
      return "运行中";
    case "degraded":
      return "受限运行";
    case "setup_required":
      return "需要设置";
    case "shutting_down":
      return "停止中";
    case "failed":
      return "启动失败";
    default:
      return "未知状态";
  }
}

function updateDraft(partial: Partial<LauncherSettings>) {
  emit("updateSettings", {
    ...(props.settingsDraft ?? props.snapshot.settings),
    ...partial,
  });
}
</script>

<template>
  <div class="app-shell">
    <aside class="shell-sidebar">
      <div class="brand-card">
        <div class="brand-eyebrow">RayleaBot</div>
        <h1>RayleaBot 启动器</h1>
        <p>本地服务壳、环境检查和管理入口。</p>
      </div>

      <nav class="section-nav" aria-label="Primary">
        <button
          v-for="section in sections"
          :key="section.id"
          class="nav-item"
          :class="{ active: activeSection === section.id }"
          type="button"
          @click="emit('navigate', section.id)"
        >
          {{ section.title }}
        </button>
      </nav>

      <div class="sidebar-summary">
        <span>当前状态</span>
        <strong>{{ trayStatus }}</strong>
      </div>
      <div class="sidebar-summary subtle">
        <span>服务入口</span>
        <strong>{{ snapshot.endpoint.baseUrl }}</strong>
      </div>
    </aside>

    <main class="shell-main">
      <section class="hero-card">
        <div class="hero-copy">
          <div class="hero-eyebrow">Service Control</div>
          <h2>{{ snapshot.serviceDetail }}</h2>
          <p>{{ snapshot.lastError || "查看当前状态、主操作和需要处理的问题。" }}</p>
        </div>
        <div class="hero-actions">
          <button class="action primary" type="button" :disabled="controlsDisabled || busyAction === 'start'" @click="emit('start')">
            {{ snapshot.serviceState === 'external_service' || snapshot.serviceState === 'ready' ? '重新检查' : '启动服务' }}
          </button>
          <button class="action" type="button" :disabled="controlsDisabled || busyAction === 'stop'" @click="emit('stop')">停止服务</button>
          <button class="action" type="button" :disabled="controlsDisabled" @click="emit('openWeb')">打开管理界面</button>
          <button class="action ghost" type="button" :disabled="controlsDisabled" @click="emit('refresh')">刷新状态</button>
        </div>
      </section>

      <section v-if="primaryIssue" class="issue-banner" :class="primaryIssue.severity">
        <div>
          <strong>{{ primaryIssue.title }}</strong>
          <p>{{ primaryIssue.summary }}</p>
        </div>
        <p>{{ primaryIssue.remediation || primaryIssue.detail }}</p>
      </section>

      <section v-if="activeSection === 'status'" class="content-grid">
        <article class="panel panel-primary">
          <h3>服务信息</h3>
          <dl class="kv-grid">
            <div><dt>状态</dt><dd>{{ trayStatus }}</dd></div>
            <div><dt>服务入口</dt><dd>{{ snapshot.endpoint.baseUrl }}</dd></div>
            <div><dt>工作目录</dt><dd>{{ snapshot.settings.workdir }}</dd></div>
            <div><dt>PID</dt><dd>{{ snapshot.processId ?? '—' }}</dd></div>
          </dl>
        </article>

        <article class="panel">
          <h3>版本与发布</h3>
          <p>{{ snapshot.releaseCheck.summary }}</p>
          <button class="action ghost" type="button" @click="emit('openReleasePage')">打开发布页</button>
        </article>

        <article class="panel full">
          <h3>最近错误输出</h3>
          <pre class="log-surface">{{ snapshot.recentStderr.join('\n') || '当前没有新的错误。' }}</pre>
          <button class="action ghost" type="button" @click="emit('openLogs')">打开日志目录</button>
        </article>
      </section>

      <section v-else-if="activeSection === 'environment'" class="content-grid">
        <article class="panel metric-panel">
          <div class="metric"><span>阻塞项</span><strong>{{ groupedChecks.blocking.length }}</strong></div>
          <div class="metric"><span>需注意</span><strong>{{ groupedChecks.warnings.length }}</strong></div>
          <div class="metric"><span>正常项</span><strong>{{ groupedChecks.ready.length }}</strong></div>
        </article>

        <article class="panel full">
          <h3>环境检查结果</h3>
          <ul class="check-list">
            <li v-for="item in snapshot.environmentChecks" :key="item.code" :class="item.severity">
              <div>
                <strong>{{ item.title }}</strong>
                <p>{{ item.summary }}</p>
              </div>
              <p>{{ item.detail }} {{ item.remediation }}</p>
            </li>
          </ul>
        </article>
      </section>

      <section v-else-if="activeSection === 'diagnostics'" class="content-grid">
        <article class="panel full">
          <h3>诊断摘要</h3>
          <pre class="log-surface">{{ diagnosticsSummary }}</pre>
        </article>
      </section>

      <section v-else class="content-grid">
        <article class="panel full">
          <div class="settings-header">
            <div>
              <h3>本地设置</h3>
              <p>管理本地路径和关闭策略。</p>
            </div>
            <div class="settings-actions">
              <button v-if="!editingSettings" class="action ghost" type="button" :disabled="controlsDisabled" @click="emit('beginEdit')">编辑路径</button>
              <template v-else>
                <button class="action ghost" type="button" :disabled="controlsDisabled" @click="emit('cancelEdit')">取消编辑</button>
                <button class="action primary" type="button" :disabled="controlsDisabled" @click="emit('saveSettings')">保存设置</button>
              </template>
            </div>
          </div>

          <div class="settings-grid">
            <label class="field">
              <span>服务端可执行文件</span>
              <div class="field-inline">
                <input :value="(settingsDraft ?? snapshot.settings).serverExecutablePath" :readonly="!editingSettings" @input="updateDraft({ serverExecutablePath: ($event.target as HTMLInputElement).value })" />
                <button class="action ghost" type="button" :disabled="!editingSettings" @click="emit('chooseServer')">浏览</button>
              </div>
            </label>

            <label class="field">
              <span>用户配置文件</span>
              <div class="field-inline">
                <input :value="(settingsDraft ?? snapshot.settings).configPath" :readonly="!editingSettings" @input="updateDraft({ configPath: ($event.target as HTMLInputElement).value })" />
                <button class="action ghost" type="button" :disabled="!editingSettings" @click="emit('chooseConfig')">浏览</button>
              </div>
            </label>

            <label class="field">
              <span>工作目录</span>
              <div class="field-inline">
                <input :value="(settingsDraft ?? snapshot.settings).workdir" :readonly="!editingSettings" @input="updateDraft({ workdir: ($event.target as HTMLInputElement).value })" />
                <button class="action ghost" type="button" :disabled="!editingSettings" @click="emit('chooseWorkdir')">选择目录</button>
              </div>
            </label>
          </div>

          <div class="close-behavior">
            <label>
              <input
                type="radio"
                name="close-behavior"
                value="ask_every_time"
                :checked="(settingsDraft ?? snapshot.settings).closeBehavior === 'ask_every_time'"
                :disabled="!editingSettings"
                @change="updateDraft({ closeBehavior: 'ask_every_time' })"
              />
              每次询问
            </label>
            <label>
              <input
                type="radio"
                name="close-behavior"
                value="hide_to_tray"
                :checked="(settingsDraft ?? snapshot.settings).closeBehavior === 'hide_to_tray'"
                :disabled="!editingSettings"
                @change="updateDraft({ closeBehavior: 'hide_to_tray' })"
              />
              隐藏到托盘
            </label>
            <label>
              <input
                type="radio"
                name="close-behavior"
                value="exit_application"
                :checked="(settingsDraft ?? snapshot.settings).closeBehavior === 'exit_application'"
                :disabled="!editingSettings"
                @change="updateDraft({ closeBehavior: 'exit_application' })"
              />
              完全退出
            </label>
          </div>

          <button class="action danger" type="button" @click="emit('exit')">完全退出启动器</button>
        </article>
      </section>
    </main>
  </div>
</template>
