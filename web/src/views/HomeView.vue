<script setup lang="ts">
import type { UserProfile } from "@/types/user";

defineProps<{
  user: UserProfile | null;
}>();

const emit = defineEmits<{
  create: [];
  join: [];
  hall: [];
  ranking: [];
  profile: [];
}>();
</script>

<template>
  <section class="view active" aria-label="首页">
    <header class="topbar">
      <a class="brand" href="#" aria-label="棋遇首页">
        <span class="brand-mark" aria-hidden="true">
          <i class="stone stone-black"></i><i class="stone stone-white"></i>
        </span>
        <span><strong>棋遇</strong><small>GOMOKU CLUB</small></span>
      </a>
      <button class="profile-chip" type="button" @click="emit('profile')">
        <span class="avatar avatar-small" aria-hidden="true">
          <span class="avatar-ear left"></span><span class="avatar-ear right"></span><span class="avatar-face">•ᴗ•</span>
        </span>
        <span class="profile-copy"><strong>{{ user?.nickname || "正在认领棋手身份" }}</strong><small>棋遇游客</small></span>
        <svg viewBox="0 0 24 24"><path d="m9 18 6-6-6-6" /></svg>
      </button>
    </header>

    <section class="welcome">
      <div>
        <span class="eyebrow">你好，{{ user?.nickname || "新棋手" }}</span>
        <h1>和朋友，<br />来一盘吧！</h1>
        <p><span class="status-dot"></span> 6 位好友正在对局</p>
      </div>
      <div class="mascot-scene" aria-hidden="true">
        <span class="sparkle sparkle-one">✦</span><span class="sparkle sparkle-two">✦</span>
        <div class="speech-bubble">等你来！</div>
        <div class="mascot mascot-black">
          <span class="mascot-ear left"></span><span class="mascot-ear right"></span>
          <span class="mascot-face"><i class="eye"></i><i class="eye"></i><b>ᴗ</b></span>
        </div>
        <div class="mascot mascot-white">
          <span class="mascot-ear left"></span><span class="mascot-ear right"></span>
          <span class="mascot-face"><i class="eye"></i><i class="eye"></i><b>ᴗ</b></span>
        </div>
      </div>
    </section>

    <section class="game-actions" aria-label="开始游戏">
      <button class="action-card friend-card" type="button" @click="emit('create')">
        <span class="action-icon" aria-hidden="true">♟</span>
        <span class="action-copy">
          <span class="action-label">好友对战</span>
          <strong>创建房间，<br />分享给朋友</strong>
          <small>无需注册 · 点击即玩</small>
        </span>
        <span class="action-arrow" aria-hidden="true">›</span>
        <span class="card-doodle" aria-hidden="true">✿</span>
      </button>
      <button class="action-card rank-card hall-card" type="button" @click="emit('hall')">
        <span class="action-icon" aria-hidden="true">♣</span>
        <span class="action-copy">
          <span class="action-label">在线大厅</span>
          <strong>看看谁在等，<br />随时加入一局</strong>
          <small>真实房间 · 实时刷新</small>
        </span>
        <span class="rank-badge">LIVE</span>
        <span class="action-arrow" aria-hidden="true">›</span>
      </button>
      <button class="join-room" type="button" @click="emit('join')">
        <span class="join-icon" aria-hidden="true">#</span>
        <span><strong>输入房间号</strong><small>已有好友在等你？</small></span>
        <svg class="chevron" viewBox="0 0 24 24"><path d="m9 18 6-6-6-6" /></svg>
      </button>
    </section>

    <section class="ranking-section">
      <div class="section-heading">
        <div><span class="section-kicker">本周棋坛</span><h2>好友排行榜</h2></div>
        <button type="button" @click="emit('ranking')">查看全部 <span>›</span></button>
      </div>
      <div class="ranking-card">
        <div class="ranking-row">
          <span class="rank-number rank-first">1</span><span class="avatar avatar-blue">小</span>
          <span class="rank-user"><strong>下棋小熊</strong><small>钻石 II · 8 连胜</small></span>
          <strong class="rank-score">1,286</strong>
        </div>
        <div class="ranking-row">
          <span class="rank-number rank-second">2</span><span class="avatar avatar-yellow">团</span>
          <span class="rank-user"><strong>糯米团</strong><small>钻石 III · 胜率 72%</small></span>
          <strong class="rank-score">1,174</strong>
        </div>
        <div class="ranking-row current-user">
          <span class="rank-number">18</span><span class="avatar avatar-small"><span class="avatar-face">•ᴗ•</span></span>
          <span class="rank-user"><strong>{{ user?.nickname || "新棋手" }} <em>我</em></strong><small>匿名身份 · 自动保存</small></span>
          <strong class="rank-score">{{ user?.rating || 1000 }}</strong>
        </div>
      </div>
    </section>
  </section>
</template>
