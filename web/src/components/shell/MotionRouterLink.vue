<script setup lang="ts">
import { computed, useAttrs } from 'vue'
import { useRouter, type RouteLocationRaw } from 'vue-router'

import { useMotionNavigation } from '@/motion/useMotionNavigation'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  to: RouteLocationRaw
}>()

const attrs = useAttrs()
const router = useRouter()
const navigate = useMotionNavigation()
const href = computed(() => router.resolve(props.to).href)

function handleClick(event: MouseEvent) {
  if (
    event.defaultPrevented
    || event.button !== 0
    || event.metaKey
    || event.altKey
    || event.ctrlKey
    || event.shiftKey
    || attrs.target === '_blank'
  ) {
    return
  }

  event.preventDefault()
  void navigate(props.to)
}
</script>

<template>
  <a v-bind="$attrs" :href="href" @click="handleClick">
    <slot />
  </a>
</template>
