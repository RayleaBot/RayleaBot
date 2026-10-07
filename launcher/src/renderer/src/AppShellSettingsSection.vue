<script setup lang="ts">
import {
  AppWindowIcon,
  CheckIcon,
  FileCogIcon,
  FolderIcon,
  FolderOpenIcon,
  FolderTreeIcon,
  KeyRoundIcon,
  LogOutIcon,
  PencilIcon,
  SaveIcon,
  ServerIcon,
  WrenchIcon,
  XIcon,
} from "@lucide/vue";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type {
  LauncherAdvancedOverrides,
  LauncherResolvedSettings,
  LauncherSettings,
  LauncherSnapshot,
} from "@shared/launcher-models";
import { RadioGroupItem, RadioGroupRoot, TabsContent, TabsList, TabsRoot, TabsTrigger, type AcceptableValue } from "reka-ui";
import { computed, ref, useId, type Component } from "vue";

import { closeBehaviorOptions } from "./AppShell.shared";
import DetailRow from "./DetailRow.vue";
import LauncherButton from "./LauncherButton.vue";

const props = defineProps<{
  snapshot: LauncherSnapshot;
  settingsDraft: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  editingSettings: boolean;
  closeBehavior: LauncherSettings["closeBehavior"];
  closeBehaviorError: string;
  busyAction: string | null;
  controlsDisabled: boolean;
}>();
const emit = defineEmits<{
  beginEdit: [];
  cancelEdit: [];
  saveSettings: [];
  updateInstallationRoot: [value: string];
  updateAdvancedOverride: [key: keyof LauncherAdvancedOverrides, value: string];
  chooseInstallationRoot: [];
  chooseServer: [];
  chooseConfig: [];
  chooseWorkdir: [];
  selectCloseBehavior: [value: LauncherSettings["closeBehavior"]];
  resetAdmin: [];
  exit: [];
}>();

type SettingsTab = "paths" | "close" | "maintenance";

const settingsTabs: Array<{ value: SettingsTab; label: string; icon: Component }> = [
  { value: "paths", label: "路径", icon: FolderTreeIcon },
  { value: "close", label: "关闭窗口", icon: AppWindowIcon },
  { value: "maintenance", label: "维护", icon: WrenchIcon },
];
const activeTab = ref<SettingsTab>("paths");
const optionId = useId();

const serverExecutablePath = computed(() => props.settingsDraft.advancedOverrides?.serverExecutablePath || props.resolvedSettings.serverExecutablePath);
const configPath = computed(() => props.settingsDraft.advancedOverrides?.configPath || props.resolvedSettings.configPath);
const workdir = computed(() => props.settingsDraft.advancedOverrides?.workdir || props.resolvedSettings.workdir);
const resetDisabled = computed(() => {
  const state = deriveLauncherPresentation(props.snapshot).state;
  return props.controlsDisabled || state === "starting" || state === "stopping";
});
// The options stay enabled through their own save, so the focused option keeps focus.
const closeBehaviorDisabled = computed(() => props.controlsDisabled && props.busyAction !== "save-close-behavior");

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

function selectCloseBehavior(value: AcceptableValue) {
  const option = closeBehaviorOptions.find((candidate) => candidate.value === value);
  if (option) emit("selectCloseBehavior", option.value);
}
</script>

<template>
  <TabsRoot v-model="activeTab" class="settings-tabs" activation-mode="automatic">
    <TabsList class="settings-tabs__list" aria-label="偏好设置分类">
      <TabsTrigger v-for="tab in settingsTabs" :key="tab.value" :value="tab.value" class="settings-tabs__trigger">
        <component :is="tab.icon" aria-hidden="true" />
        {{ tab.label }}
      </TabsTrigger>
    </TabsList>

    <TabsContent value="paths" class="settings-panel">
      <div class="settings-panel__header">
        <p class="settings-panel__description">
          {{ editingSettings ? "当前内容是草稿，保存后生效。" : "启动器当前使用的目录和文件位置。" }}
        </p>
        <div class="settings-panel__actions">
          <template v-if="editingSettings">
            <LauncherButton :icon="XIcon" :disabled="controlsDisabled" @click="emit('cancelEdit')">放弃</LauncherButton>
            <LauncherButton
              :icon="SaveIcon"
              :emphasis="controlsDisabled ? 'regular' : 'prominent'"
              :disabled="controlsDisabled"
              @click="emit('saveSettings')"
            >
              保存
            </LauncherButton>
          </template>
          <LauncherButton v-else :icon="PencilIcon" :disabled="controlsDisabled" @click="emit('beginEdit')">编辑路径</LauncherButton>
        </div>
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
    </TabsContent>

    <TabsContent value="close" class="settings-panel">
      <div class="settings-panel__header">
        <p :id="`${optionId}-description`" class="settings-panel__description">关闭启动器窗口时执行的操作，选择后立即生效。</p>
      </div>

      <RadioGroupRoot
        class="close-options"
        :model-value="closeBehavior"
        :disabled="closeBehaviorDisabled"
        aria-label="关闭窗口时的操作"
        :aria-describedby="`${optionId}-description`"
        @update:model-value="selectCloseBehavior"
      >
        <RadioGroupItem
          v-for="option in closeBehaviorOptions"
          :key="option.value"
          class="close-option"
          :value="option.value"
          :data-tone="option.tone"
          :aria-labelledby="`${optionId}-${option.value}`"
          :aria-describedby="`${optionId}-${option.value}-detail`"
        >
          <span class="close-option__icon" aria-hidden="true"><component :is="option.icon" /></span>
          <span class="close-option__mark" aria-hidden="true"><CheckIcon /></span>
          <span :id="`${optionId}-${option.value}`" class="close-option__title">{{ option.label }}</span>
          <span :id="`${optionId}-${option.value}-detail`" class="close-option__detail">{{ option.detail }}</span>
        </RadioGroupItem>
      </RadioGroupRoot>

      <div v-if="closeBehaviorError" class="attention-note" data-severity="danger" role="alert">
        <strong>关闭方式没有保存</strong>
        <span>{{ closeBehaviorError }}</span>
      </div>
    </TabsContent>

    <TabsContent value="maintenance" class="settings-panel">
      <div class="settings-panel__header">
        <p class="settings-panel__description">重置管理员账号或退出启动器。</p>
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
    </TabsContent>
  </TabsRoot>
</template>
