<script setup lang="ts">
import { computed } from "vue";
import type { UserProfile } from "@/types/user";

const props = defineProps<{
  user: UserProfile | null;
}>();

const emit = defineEmits<{
  edit: [];
}>();

const totalGames = computed(
  () => (props.user?.wins || 0) + (props.user?.losses || 0) + (props.user?.draws || 0),
);
const winRate = computed(() =>
  totalGames.value ? `${Math.round(((props.user?.wins || 0) / totalGames.value) * 100)}%` : "—",
);
const joinedDays = computed(() => {
  if (!props.user?.createdAt) return 1;
  return Math.max(1, Math.floor((Date.now() - props.user.createdAt) / 86400000) + 1);
});
const shortID = computed(() => props.user?.id.replace(/^QY/, "") || "--------");
</script>

<template>
  <section class="view page-view profile-page active" aria-label="我的">
    <header class="simple-header">
      <div><span class="section-kicker">PLAYER PROFILE</span><h1>我的棋室</h1></div>
      <button class="profile-edit-button" type="button" @click="emit('edit')">
        <span aria-hidden="true">✎</span> 修改昵称
      </button>
    </header>
    <div class="profile-hero">
      <span
        class="profile-main-avatar"
        :class="`user-avatar-${user?.avatar || 'peach-cat'}`"
      >
        <i class="cat-ear left"></i><i class="cat-ear right"></i><b>• ᴗ •</b>
      </span>
      <div>
        <h2>{{ user?.nickname || "正在加载棋手身份" }}</h2>
        <p>棋遇号 {{ shortID }} · 加入第 {{ joinedDays }} 天</p>
        <span class="level-tag">匿名棋手</span>
      </div>
    </div>
    <div class="identity-safe-tip">
      <span aria-hidden="true">⌁</span>
      <div><strong>身份已自动保存</strong><small>同一浏览器再次打开时，会自动认出你。</small></div>
    </div>
    <div class="stats-grid">
      <div><strong>{{ totalGames }}</strong><small>总对局</small></div>
      <div><strong>{{ winRate }}</strong><small>胜率</small></div>
      <div><strong>{{ user?.draws || 0 }}</strong><small>和棋</small></div>
    </div>
    <div class="level-card">
      <div class="level-title">
        <span><small>当前积分</small><strong>{{ user?.rating || 1000 }}</strong></span>
        <em>游客资料已落盘保存</em>
      </div>
      <div class="level-progress"><i style="width: 18%"></i></div>
      <div class="level-scale"><span>新晋棋手</span><span>继续挑战</span></div>
    </div>
    <section class="history-section">
      <div class="section-heading">
        <div><span class="section-kicker">RECENT GAMES</span><h2>最近对局</h2></div>
      </div>
      <div class="profile-empty-history">
        <span aria-hidden="true">♟</span>
        <strong>还没有记录的对局</strong>
        <small>完成棋局后，战绩会出现在这里。</small>
      </div>
    </section>
  </section>
</template>
