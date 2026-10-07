<script setup lang="ts">
import type { Component } from "vue";

withDefaults(defineProps<{
  icon: Component;
  label: string;
  value?: string;
  title?: string;
  mono?: boolean;
}>(), { mono: true });
</script>

<template>
  <div class="detail-list__row">
    <dt>
      <span class="detail-list__icon" aria-hidden="true"><component :is="icon" /></span>
      {{ label }}
    </dt>
    <dd>
      <slot>
        <component :is="mono ? 'code' : 'span'" class="detail-list__value" :title="title">{{ value }}</component>
      </slot>
    </dd>
  </div>
</template>
