<template>
  <div class="auth-layout">
    <section class="auth-layout__hero">
      <div class="auth-layout__hero-inner">
        <div class="auth-layout__eyebrow">RayleaBot Console</div>
        <h1>Vue Vben Admin 风格管理界面</h1>
        <p>统一登录、初始化和后台工作台入口，保留当前系统状态、任务流和协议行为。</p>

        <div class="auth-layout__highlights">
          <article>
            <strong>统一壳体</strong>
            <span>菜单、页签、面包屑和主题切换采用一致的后台框架。</span>
          </article>
          <article>
            <strong>业务语义不变</strong>
            <span>登录、会话、任务、日志、协议与插件行为继续沿用正式契约。</span>
          </article>
          <article>
            <strong>桌面与移动端兼容</strong>
            <span>认证页和管理页都按单应用结构适配桌面和窄屏。</span>
          </article>
        </div>
      </div>
    </section>

    <section class="auth-layout__panel">
      <RouterView />
    </section>
  </div>
</template>
