<script setup lang="ts">defineProps<{ label: string }>()</script>
<template><div class="app-detail-item"><dt>{{ label }}</dt><dd><slot /></dd></div></template>
<style>
.app-detail-item { display: grid; grid-template-columns: minmax(110px, 30%) minmax(0, 1fr); border-bottom: 1px solid var(--border); font-size: 13px; line-height: 1.6; }
.app-detail-item:last-child { border-bottom: 0; }
.app-detail-item dt { padding: 10px 12px; color: var(--muted); background: var(--surface-soft); }
.app-detail-item dd { min-width: 0; margin: 0; padding: 10px 12px; overflow-wrap: anywhere; }
</style>
