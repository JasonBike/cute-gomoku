<script setup lang="ts">
export type AppView = "home" | "hall" | "ranking" | "profile";

defineProps<{ current: AppView }>();
const emit = defineEmits<{ navigate: [view: AppView] }>();
</script>

<template>
  <nav class="bottom-nav" aria-label="主导航">
    <button class="nav-item" :class="{ active: current === 'home' }" type="button" @click="emit('navigate', 'home')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><path d="m3 11 9-8 9 8v9h-6v-6H9v6H3z" /></svg>
      </span>
      <span>首页</span>
    </button>
    <button class="nav-item" :class="{ active: current === 'hall' }" type="button" @click="emit('navigate', 'hall')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><path d="M4 20V8l8-5 8 5v12M4 10h16M8 14h2M14 14h2M9 20v-3h6v3" /></svg>
      </span>
      <span>大厅</span>
    </button>
    <button class="nav-item" :class="{ active: current === 'ranking' }" type="button" @click="emit('navigate', 'ranking')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><path d="M5 20V10h4v10H5Zm5 0V4h4v16h-4Zm5 0v-7h4v7h-4Z" /></svg>
      </span>
      <span>排行榜</span>
    </button>
    <button class="nav-item" :class="{ active: current === 'profile' }" type="button" @click="emit('navigate', 'profile')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="8" r="4" /><path d="M4 21c.8-4.2 3.5-6.3 8-6.3s7.2 2.1 8 6.3" /></svg>
      </span>
      <span>我的</span>
    </button>
  </nav>
</template>
