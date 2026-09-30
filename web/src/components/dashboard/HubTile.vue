<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'

import MotionRouterLink from '@/components/shell/MotionRouterLink.vue'

export type HubTileGlyph = 'amber' | 'blue' | 'ember' | 'green' | 'rose' | 'slate' | 'teal' | 'violet' | 'image'
export type HubTileTone = 'success' | 'warning' | 'danger' | 'info' | 'muted'

// One object on the control hub. Objects that are running, connected or ready are lit tiles that stay
// light in both themes; stopped objects and actions are clear glass on the light field. `attention`
// outlines a tile whose object needs a decision without changing whether it is lit.
withDefaults(defineProps<{
  lit?: boolean
  wide?: boolean
  glyph?: HubTileGlyph
  tone?: HubTileTone
  attention?: boolean
  to?: RouteLocationRaw
  action?: boolean
  disabled?: boolean
  label?: string
}>(), { lit: false, wide: false, glyph: 'slate', tone: 'muted', attention: false, action: false, disabled: false })

defineEmits<{ click: [event: MouseEvent] }>()
</script>

<template>
  <MotionRouterLink
    v-if="to"
    :to="to"
    :aria-label="label"
    :class="['hub-tile', lit ? 'hub-tile--lit' : 'hub-tile--glass liquid-glass', { 'hub-tile--wide': wide, 'hub-tile--attention': attention }]"
    :data-glass="lit ? undefined : 'clear'"
    :data-tone="tone"
  >
    <span class="hub-tile__glyph" :data-glyph="glyph" aria-hidden="true"><slot name="glyph" /></span>
    <span class="hub-tile__body">
      <strong class="hub-tile__title"><slot name="title" /></strong>
      <span v-if="$slots.detail" class="hub-tile__detail"><slot name="detail" /></span>
      <span class="hub-tile__status"><slot name="status" /></span>
    </span>
  </MotionRouterLink>
  <button
    v-else-if="action"
    type="button"
    :disabled="disabled"
    :aria-label="label"
    :class="['hub-tile', lit ? 'hub-tile--lit' : 'hub-tile--glass liquid-glass', { 'hub-tile--wide': wide, 'hub-tile--attention': attention }]"
    :data-glass="lit ? undefined : 'clear'"
    :data-tone="tone"
    @click="$emit('click', $event)"
  >
    <span class="hub-tile__glyph" :data-glyph="glyph" aria-hidden="true"><slot name="glyph" /></span>
    <span class="hub-tile__body">
      <strong class="hub-tile__title"><slot name="title" /></strong>
      <span v-if="$slots.detail" class="hub-tile__detail"><slot name="detail" /></span>
      <span class="hub-tile__status"><slot name="status" /></span>
    </span>
  </button>
  <div
    v-else
    :class="['hub-tile', lit ? 'hub-tile--lit' : 'hub-tile--glass liquid-glass', { 'hub-tile--wide': wide, 'hub-tile--attention': attention }]"
    :data-glass="lit ? undefined : 'clear'"
    :data-tone="tone"
  >
    <span class="hub-tile__glyph" :data-glyph="glyph" aria-hidden="true"><slot name="glyph" /></span>
    <span class="hub-tile__body">
      <strong class="hub-tile__title"><slot name="title" /></strong>
      <span v-if="$slots.detail" class="hub-tile__detail"><slot name="detail" /></span>
      <span class="hub-tile__status"><slot name="status" /></span>
    </span>
    <slot name="footer" />
  </div>
</template>

<style scoped lang="scss">
.hub-tile {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
  min-height: 112px;
  padding: 14px 14px 13px 15px;
  border: 0;
  border-radius: var(--app-tile-radius);
  color: var(--text);
  font: inherit;
  text-align: left;
  text-decoration: none;
  transition: translate var(--motion-fast, 160ms) cubic-bezier(.16, 1, .3, 1), box-shadow var(--motion-fast, 160ms);
}

.hub-tile--wide {
  grid-column: span 2;
  flex-direction: row;
  align-items: flex-start;
  gap: 14px;
}

.hub-tile--lit {
  background: var(--surface-lit);
  color: var(--on-lit);
  box-shadow:
    inset 0 1.5px 0 var(--glass-rim),
    inset 0 0 0 1px color-mix(in srgb, var(--glass-rim) 50%, transparent),
    0 1px 2px rgb(0 0 0 / 6%),
    0 12px 28px -12px rgb(0 0 0 / 22%);
}

:global([data-theme='dark']) .hub-tile--lit {
  box-shadow:
    inset 0 1.5px 0 var(--glass-rim),
    0 12px 30px -12px rgb(0 0 0 / 60%);
}

.hub-tile--attention.hub-tile--lit {
  box-shadow:
    inset 0 0 0 2px var(--lit-warning),
    0 12px 28px -12px rgb(0 0 0 / 22%);
}

.hub-tile--attention.hub-tile--glass {
  box-shadow:
    inset 0 0 0 2px var(--warning),
    var(--shadow-floating);
}

.hub-tile--attention[data-tone='danger'].hub-tile--lit {
  box-shadow:
    inset 0 0 0 2px var(--lit-danger),
    0 12px 28px -12px rgb(0 0 0 / 22%);
}

.hub-tile--attention[data-tone='danger'].hub-tile--glass {
  box-shadow:
    inset 0 0 0 2px var(--danger),
    var(--shadow-floating);
}

a.hub-tile,
button.hub-tile {
  cursor: pointer;
}

a.hub-tile:hover,
button.hub-tile:not(:disabled):hover {
  translate: 0 -1px;
}

button.hub-tile:disabled {
  cursor: progress;
}

// Tiles are large objects on the field, so their focus ring sits outside the tile where it contrasts
// with the field in both themes instead of disappearing into a lit surface.
.hub-tile:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: 3px;
}

.hub-tile__glyph {
  display: grid;
  flex: none;
  place-items: center;
  width: 40px;
  height: 40px;
  overflow: hidden;
  border-radius: 13px;
  background: var(--tile-glyph-slate);
  color: var(--on-tile-glyph);
  font-size: 19px;
  font-weight: 700;
  box-shadow: inset 0 1px 0 rgb(255 255 255 / 40%);
}

.hub-tile--wide .hub-tile__glyph {
  width: 52px;
  height: 52px;
  border-radius: 16px;
}

.hub-tile__glyph :deep(svg) {
  width: 20px;
  height: 20px;
  stroke-width: 2;
}

.hub-tile--wide .hub-tile__glyph :deep(svg) {
  width: 24px;
  height: 24px;
}

.hub-tile__glyph[data-glyph='amber'] { background: var(--tile-glyph-amber); }
.hub-tile__glyph[data-glyph='blue'] { background: var(--tile-glyph-blue); }
.hub-tile__glyph[data-glyph='ember'] { background: var(--tile-glyph-ember); }
.hub-tile__glyph[data-glyph='green'] { background: var(--tile-glyph-green); }
.hub-tile__glyph[data-glyph='rose'] { background: var(--tile-glyph-rose); }
.hub-tile__glyph[data-glyph='teal'] { background: var(--tile-glyph-teal); }
.hub-tile__glyph[data-glyph='violet'] { background: var(--tile-glyph-violet); }
// A plugin's own icon sits on the lit surface colour instead of an identity fill.
.hub-tile__glyph[data-glyph='image'] { background: var(--surface-lit); box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--on-lit) 10%, transparent); }

// Unlit objects lose their identity colour: the glyph becomes part of the glass.
.hub-tile--glass .hub-tile__glyph {
  background: color-mix(in srgb, var(--glass-rim) 55%, transparent);
  color: var(--muted);
  box-shadow: inset 0 1px 0 var(--glass-rim);
}

.hub-tile__body {
  display: flex;
  flex: 1;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.hub-tile__title {
  overflow: hidden;
  font-size: 15px;
  font-weight: 700;
  line-height: 1.35;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hub-tile:not(.hub-tile--wide) .hub-tile__body {
  justify-content: flex-end;
}

.hub-tile__detail {
  overflow: hidden;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.45;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.hub-tile__status {
  display: flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
  color: var(--muted);
  font-size: 12.5px;
  font-weight: 500;
  line-height: 1.4;
}

.hub-tile--wide .hub-tile__status {
  margin-top: auto;
  padding-top: 4px;
}

.hub-tile__status :deep(svg) {
  flex: none;
  width: 14px;
  height: 14px;
  stroke-width: 2.2;
}

.hub-tile--lit .hub-tile__detail,
.hub-tile--lit .hub-tile__status {
  color: var(--on-lit-muted);
}

.hub-tile--lit[data-tone='success'] .hub-tile__status { color: var(--lit-success); font-weight: 600; }
.hub-tile--lit[data-tone='warning'] .hub-tile__status { color: var(--lit-warning); font-weight: 600; }
.hub-tile--lit[data-tone='danger'] .hub-tile__status { color: var(--lit-danger); font-weight: 600; }
.hub-tile--lit[data-tone='info'] .hub-tile__status { color: var(--lit-info); font-weight: 600; }
.hub-tile--glass[data-tone='success'] .hub-tile__status { color: var(--text-success); font-weight: 600; }
.hub-tile--glass[data-tone='warning'] .hub-tile__status { color: var(--text-warning); font-weight: 600; }
.hub-tile--glass[data-tone='danger'] .hub-tile__status { color: var(--text-danger); font-weight: 600; }
.hub-tile--glass[data-tone='info'] .hub-tile__status { color: var(--text-info); font-weight: 600; }

@media (prefers-reduced-motion: reduce) {
  .hub-tile { transition: none; }
  a.hub-tile:hover,
  button.hub-tile:not(:disabled):hover { translate: none; }
}

@media (forced-colors: active) {
  .hub-tile--lit { border: 2px solid CanvasText; }
  .hub-tile--attention { outline: 2px solid Highlight; outline-offset: -4px; }
}
</style>
