<script setup lang="ts">
import zhCN from 'ant-design-vue/es/locale/zh_CN'

import { appTheme } from '@/preferences/theme'
</script>

<template>
  <a-config-provider :locale="zhCN" :theme="appTheme">
    <a-app>
      <RouterView />
    </a-app>
  </a-config-provider>
</template>
