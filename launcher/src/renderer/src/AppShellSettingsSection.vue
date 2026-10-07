<script setup lang="ts">
import { FileCogIcon, FolderIcon, FolderOpenIcon, KeyRoundIcon, LogOutIcon, ServerIcon } from "@lucide/vue";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";
import { RadioGroupIndicator, RadioGroupItem, RadioGroupRoot, type AcceptableValue } from "reka-ui";
import { computed, type Component } from "vue";

import { closeBehaviorOptions } from "./AppShell.shared";
import DetailRow from "./DetailRow.vue";
import LauncherButton from "./LauncherButton.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  settingsDraft: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  editingSettings: boolean;
  busyAction: string | null;
  controlsDisabled: boolean;
}>();
const emit = defineEmits<{
  updateInstallationRoot: [value: string];
  updateCloseBehavior: [value: LauncherSettings["closeBehavior"]];
  updateAdvancedOverride: [key: keyof LauncherAdvancedOverrides, value: string];
  chooseInstallationRoot: [];
  chooseServer: [];
  chooseConfig: [];
  chooseWorkdir: [];
  resetAdmin: [];
  exit: [];
}>();

const serverExecutablePath = computed(() => props.settingsDraft.advancedOverrides?.serverExecutablePath || props.resolvedSettings.serverExecutablePath);
const configPath = computed(() => props.settingsDraft.advancedOverrides?.configPath || props.resolvedSettings.configPath);
const workdir = computed(() => props.settingsDraft.advancedOverrides?.workdir || props.resolvedSettings.workdir);
const closeBehavior = computed(() =>
  closeBehaviorOptions.find((option) => option.value === props.settingsDraft.closeBehavior) ?? closeBehaviorOptions[0]);
const resetDisabled = computed(() => {
  const state = deriveLauncherPresentation(props.snapshot).state;
  return props.controlsDisabled || state === "starting" || state === "stopping";
});

const pathFields = computed((): Array<{
  icon: Component;
  label: string;
  value: string;
  update: (value: string) => void;
  choose: () => void;
}> => [
  {
    icon: FolderIcon,
    label: "安装目录",
    value: props.settingsDraft.installationRoot,
    update: (value) => emit("updateInstallationRoot", value),
    choose: () => emit("chooseInstallationRoot"),
  },
  {
    icon: ServerIcon,
    label: "服务端程序",
    value: serverExecutablePath.value,
    update: (value) => emit("updateAdvancedOverride", "serverExecutablePath", value),
    choose: () => emit("chooseServer"),
  },
  {
    icon: FileCogIcon,
    label: "配置文件",
    value: configPath.value,
    update: (value) => emit("updateAdvancedOverride", "configPath", value),
    choose: () => emit("chooseConfig"),
  },
  {
    icon: FolderOpenIcon,
    label: "工作目录",
    value: workdir.value,
    update: (value) => emit("updateAdvancedOverride", "workdir", value),
    choose: () => emit("chooseWorkdir"),
  },
]);

function displayPath(value: string) {
  return value.trim() || "未设置";
}

function updateCloseBehavior(value: AcceptableValue) {
  const option = closeBehaviorOptions.find((candidate) => candidate.value === value);
  if (option) emit("updateCloseBehavior", option.value);
}
</script>

<template>
  <div class="settings-workspace" :data-busy="busyAction ?? 'idle'">
    <div v-if="editingSettings" class="attention-note" role="status">
      <strong>正在编辑设置</strong>
      <span>当前内容是草稿，保存后生效。</span>
    </div>

    <section class="workspace-group" aria-labelledby="settings-paths-title">
      <div class="workspace-group__header">
        <h3 id="settings-paths-title" class="workspace-group__title">路径设置</h3>
        <p class="workspace-group__description">启动器当前使用的目录和文件位置。</p>
      </div>

      <div v-if="editingSettings" class="field-list content-group">
        <div v-for="field in pathFields" :key="field.label" class="field-row">
          <span class="field-row__label">
            <span class="detail-list__icon" aria-hidden="true"><component :is="field.icon" /></span>
            {{ field.label }}
          </span>
          <div class="field-row__control">
            <input
              class="settings-input settings-input--path"
              :aria-label="field.label"
              :value="field.value"
              :disabled="controlsDisabled"
              spellcheck="false"
              @input="field.update(($event.target as HTMLInputElement).value)"
            >
            <LauncherButton :icon="FolderOpenIcon" :disabled="controlsDisabled" @click="field.choose">浏览</LauncherButton>
          </div>
        </div>
      </div>
      <dl v-else class="detail-list detail-list--wrap content-group">
        <DetailRow
          v-for="field in pathFields"
          :key="field.label"
          :icon="field.icon"
          :label="field.label"
          :value="displayPath(field.value)"
          :title="field.value || undefined"
        />
      </dl>
    </section>

    <section class="workspace-group" aria-labelledby="settings-close-title">
      <div class="workspace-group__header">
        <h3 id="settings-close-title" class="workspace-group__title">关闭行为</h3>
        <p class="workspace-group__description">关闭窗口时采用的默认动作。</p>
      </div>

      <RadioGroupRoot
        v-if="editingSettings"
        class="choice-list content-group"
        :model-value="settingsDraft.closeBehavior"
        :disabled="controlsDisabled"
        aria-labelledby="settings-close-title"
        @update:model-value="updateCloseBehavior"
      >
        <label
          v-for="option in closeBehaviorOptions"
          :key="option.value"
          class="choice-row"
          :data-selected="settingsDraft.closeBehavior === option.value"
        >
          <RadioGroupItem class="choice-row__radio" :value="option.value">
            <RadioGroupIndicator class="choice-row__radio-dot" />
          </RadioGroupItem>
          <span class="choice-row__body">
            <span class="choice-row__title">{{ option.label }}</span>
            <span class="choice-row__detail">{{ option.detail }}</span>
          </span>
        </label>
      </RadioGroupRoot>
      <div v-else class="choice-summary content-group">
        <strong>{{ closeBehavior.label }}</strong>
        <span>{{ closeBehavior.detail }}</span>
      </div>
    </section>

    <section class="workspace-group" aria-labelledby="settings-maintenance-title">
      <div class="workspace-group__header">
        <h3 id="settings-maintenance-title" class="workspace-group__title">维护操作</h3>
        <p class="workspace-group__description">用于重置管理员账号或退出启动器。</p>
      </div>

      <div class="action-list content-group">
        <div class="action-row" data-tone="danger">
          <span class="action-row__icon" aria-hidden="true"><KeyRoundIcon /></span>
          <div class="action-row__copy">
            <strong>重置管理员账号</strong>
            <span>清除管理员账号和登录会话，随后重启服务并打开管理界面重新创建管理员；配置、数据和已安装插件保留。</span>
          </div>
          <LauncherButton class="launcher-button--danger" :disabled="resetDisabled" @click="emit('resetAdmin')">立即重置</LauncherButton>
        </div>
        <div class="action-row">
          <span class="action-row__icon" aria-hidden="true"><LogOutIcon /></span>
          <div class="action-row__copy">
            <strong>退出启动器</strong>
            <span>关闭启动器；由启动器启动的服务会一并停止。</span>
          </div>
          <LauncherButton class="launcher-button--danger" :disabled="controlsDisabled" @click="emit('exit')">退出启动器</LauncherButton>
        </div>
      </div>
    </section>
  </div>
</template>
