<script setup lang="ts">
export type AppView = "home" | "game" | "ranking" | "profile";

defineProps<{ current: AppView }>();
defineEmits<{ navigate: [view: AppView] }>();
</script>

<template>
  <nav class="bottom-nav" aria-label="主导航">
    <button class="nav-item" :class="{ active: current === 'home' }" type="button" @click="$emit('navigate', 'home')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><path d="m3 11 9-8 9 8v9h-6v-6H9v6H3z" /></svg>
      </span>
      <span>首页</span>
    </button>
    <button class="nav-item" :class="{ active: current === 'game' }" type="button" @click="$emit('navigate', 'game')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><path d="M8 3h8v4a4 4 0 0 1-8 0V3ZM8 5H4v2a4 4 0 0 0 4 4M16 5h4v2a4 4 0 0 1-4 4M12 11v5M8 21h8M9 17h6" /></svg>
      </span>
      <span>排位</span>
    </button>
    <button class="nav-item" :class="{ active: current === 'ranking' }" type="button" @click="$emit('navigate', 'ranking')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><path d="M5 20V10h4v10H5Zm5 0V4h4v16h-4Zm5 0v-7h4v7h-4Z" /></svg>
      </span>
      <span>排行榜</span>
    </button>
    <button class="nav-item" :class="{ active: current === 'profile' }" type="button" @click="$emit('navigate', 'profile')">
      <span class="nav-icon">
        <svg viewBox="0 0 24 24"><circle cx="12" cy="8" r="4" /><path d="M4 21c.8-4.2 3.5-6.3 8-6.3s7.2 2.1 8 6.3" /></svg>
      </span>
      <span>我的</span>
    </button>
  </nav>
</template>
