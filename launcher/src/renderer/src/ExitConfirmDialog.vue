<script setup lang="ts">
import { CheckIcon, ChevronRightIcon, LogOutIcon, MinusIcon, XIcon } from "@lucide/vue";
import { CheckboxIndicator, CheckboxRoot, DialogDescription, DialogTitle } from "reka-ui";
import { ref, useId } from "vue";

import LauncherButton from "./LauncherButton.vue";
import LauncherDialog from "./LauncherDialog.vue";
import StatusLens from "./StatusLens.vue";

defineProps<{
  open: boolean;
}>();
const emit = defineEmits<{
  close: [];
  confirm: [action: "hide" | "exit", setAsDefault: boolean];
}>();

const setAsDefault = ref(false);
const rememberId = useId();

function choose(action: "hide" | "exit") {
  emit("confirm", action, setAsDefault.value);
  setAsDefault.value = false;
}

function close() {
  setAsDefault.value = false;
  emit("close");
}
</script>

<template>
  <LauncherDialog :open="open" @dismiss="close">
    <div class="launcher-dialog__body">
      <DialogTitle class="launcher-dialog__title">
        <StatusLens tone="attention" size="small" :icon="XIcon" />
        <span class="launcher-dialog__heading">
          <strong>关闭启动器</strong>
          <span>选择窗口关闭后的运行方式</span>
        </span>
      </DialogTitle>
      <div class="launcher-dialog__content">
        <DialogDescription class="launcher-dialog__detail">
          启动器可以留在托盘，服务继续运行；完全退出时，由启动器启动的服务会一并停止。
        </DialogDescription>
        <div class="launcher-dialog__remember">
          <CheckboxRoot :id="rememberId" v-model="setAsDefault" class="launcher-checkbox">
            <CheckboxIndicator class="launcher-checkbox__indicator">
              <CheckIcon aria-hidden="true" />
            </CheckboxIndicator>
          </CheckboxRoot>
          <label :for="rememberId">记住本次选择，后续关闭窗口时直接执行</label>
        </div>
        <div class="launcher-dialog__choices">
          <button type="button" class="launcher-tile" @click="choose('hide')">
            <span class="launcher-tile__icon" aria-hidden="true"><MinusIcon /></span>
            <span class="launcher-tile__copy">
              <strong>隐藏到托盘</strong>
              <span>关闭窗口后启动器留在托盘，服务继续运行。</span>
            </span>
            <ChevronRightIcon class="launcher-tile__chevron" aria-hidden="true" />
          </button>
          <button type="button" class="launcher-tile" data-tone="danger" @click="choose('exit')">
            <span class="launcher-tile__icon" aria-hidden="true"><LogOutIcon /></span>
            <span class="launcher-tile__copy">
              <strong>完全退出</strong>
              <span>关闭启动器；由启动器启动的服务会一并停止。</span>
            </span>
            <ChevronRightIcon class="launcher-tile__chevron" aria-hidden="true" />
          </button>
        </div>
      </div>
      <div class="launcher-dialog__actions">
        <LauncherButton class="launcher-dialog__button" @click="close">取消</LauncherButton>
      </div>
    </div>
  </LauncherDialog>
</template>
