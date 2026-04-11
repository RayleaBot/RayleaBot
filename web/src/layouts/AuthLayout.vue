<template>
  <div class="auth-layout">
    <section class="auth-layout__hero">
      <div class="auth-layout__hero-panel">
        <span class="auth-layout__brand-badge">R</span>
        <div class="auth-layout__hero-copy">
          <div class="page-eyebrow">RayleaBot</div>
          <h1>RayleaBot</h1>
          <p>自托管聊天机器人管理界面。</p>
        </div>
        <p class="auth-layout__support">登录后可查看系统状态、插件、任务和协议日志。</p>
      </div>
    </section>

    <section class="auth-layout__panel">
      <RouterView />
    </section>
  </div>
</template>
