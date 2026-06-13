<template>
  <div class="aurora-bg" aria-hidden="true">
    <div class="aurora-bg__static" />
  </div>
</template>

<style scoped lang="scss">
.aurora-bg {
  position: absolute;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
}

.aurora-bg__static {
  position: absolute;
  inset: 0;
  background:
    radial-gradient(120% 90% at 15% 0%, color-mix(in srgb, var(--aurora-1) 32%, transparent), transparent 60%),
    radial-gradient(110% 80% at 85% 12%, color-mix(in srgb, var(--aurora-3) 26%, transparent), transparent 62%),
    radial-gradient(120% 100% at 50% 100%, color-mix(in srgb, var(--aurora-2) 20%, transparent), transparent 58%),
    var(--aurora-bg);
}
</style>
