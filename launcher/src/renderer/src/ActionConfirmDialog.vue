<script setup lang="ts">
import { CircleStopIcon, DownloadIcon, Trash2Icon } from "@lucide/vue";
import { DialogDescription, DialogTitle } from "reka-ui";
import { computed, ref, watch, type Component } from "vue";

import LauncherButton from "./LauncherButton.vue";
import LauncherDialog from "./LauncherDialog.vue";
import StatusLens from "./StatusLens.vue";
import type { ConfirmedLauncherAction } from "./useLauncherConfirmations";

const props = defineProps<{
  action: ConfirmedLauncherAction | null;
}>();
const emit = defineEmits<{
  cancel: [];
  confirm: [action: ConfirmedLauncherAction];
}>();

const actionCopy = {
  "reset-admin": {
    title: "重置管理员账号",
    lead: "确认重置管理员账号？",
    detail: "清除管理员账号和登录会话，随后重启服务并打开管理界面重新创建管理员；配置、数据和已安装插件保留。",
    confirm: "确认重置",
    icon: Trash2Icon,
    tone: "danger",
  },
  "stop-external": {
    title: "停止现有服务",
    lead: "该服务由其他进程启动，启动器无法直接停止它。",
    detail: "确认后会打开管理界面，请在那里停止服务，停止后回到启动器继续操作。",
    confirm: "打开管理界面",
    icon: CircleStopIcon,
    tone: "attention",
  },
  "apply-update": {
    title: "安装更新",
    lead: "确认下载并安装新版本？",
    detail: "启动器会停止服务，覆盖安装目录中的程序文件后重新打开；配置、数据和已安装插件保持不变。建议先创建备份。",
    confirm: "安装更新",
    icon: DownloadIcon,
    tone: "attention",
  },
} satisfies Record<ConfirmedLauncherAction, {
  title: string;
  lead: string;
  detail: string;
  confirm: string;
  icon: Component;
  tone: "danger" | "attention";
}>;

// The dialog keeps the last action's copy while it animates out.
const shownAction = ref<ConfirmedLauncherAction>(props.action ?? "reset-admin");
watch(() => props.action, (action) => {
  if (action !== null) shownAction.value = action;
});
const copy = computed(() => actionCopy[props.action ?? shownAction.value]);

function confirm() {
  if (props.action) emit("confirm", props.action);
}
</script>

<template>
  <LauncherDialog :open="action !== null" :tone="copy.tone" @dismiss="emit('cancel')">
    <div class="launcher-dialog__body">
      <DialogTitle class="launcher-dialog__title">
        <StatusLens :tone="copy.tone" size="small" :icon="copy.icon" />
        <span class="launcher-dialog__heading">
          <strong>{{ copy.title }}</strong>
        </span>
      </DialogTitle>
      <DialogDescription as="div" class="launcher-dialog__content">
        <p class="launcher-dialog__lead">{{ copy.lead }}</p>
        <p class="launcher-dialog__detail">{{ copy.detail }}</p>
      </DialogDescription>
      <div class="launcher-dialog__actions">
        <LauncherButton class="launcher-dialog__button" @click="emit('cancel')">取消</LauncherButton>
        <LauncherButton
          class="launcher-dialog__button launcher-dialog__button--confirm"
          :data-tone="copy.tone"
          @click="confirm"
        >
          {{ copy.confirm }}
        </LauncherButton>
      </div>
    </div>
  </LauncherDialog>
</template>
