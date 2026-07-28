<script setup lang="ts">
import { computed } from "vue";
import type { PlayerState } from "@/types/game";

const props = defineProps<{
  roomCode: string;
  players: PlayerState[];
  connection: "idle" | "connecting" | "online" | "offline";
}>();

defineEmits<{
  back: [];
  share: [];
  copy: [];
}>();

const blackPlayer = computed(() => props.players.find((player) => player.color === 1));
const whitePlayer = computed(() => props.players.find((player) => player.color === 2));
const message = computed(() => {
  if (props.connection === "offline") return "连接中断，正在自动重连…";
  if (props.connection === "connecting") return "正在连接房间…";
  if (whitePlayer.value) return "好友已经进入，正在准备开局…";
  return "正在等待朋友从邀请链接进入…";
});
</script>

<template>
  <section class="view page-view active" aria-label="好友房间">
    <header class="page-topbar">
      <button class="icon-button" type="button" aria-label="返回首页" @click="$emit('back')">‹</button>
      <div><strong>好友房间</strong><small>仅受邀好友可加入</small></div>
      <button class="icon-button" type="button" aria-label="更多">•••</button>
    </header>

    <div class="lobby-status">
      <span class="section-kicker">ROOM {{ roomCode || "------" }}</span>
      <h2>棋盘已经摆好啦</h2>
      <p>{{ message }}</p>
      <div class="lobby-players">
        <div class="lobby-player ready">
          <span class="big-avatar peach-cat"><i class="cat-ear left"></i><i class="cat-ear right"></i><b>• ᴗ •</b></span>
          <strong>{{ blackPlayer?.name || "小桃子" }}</strong>
          <small><i></i> {{ blackPlayer?.connected ? "已准备" : "连接中" }}</small>
        </div>
        <div class="versus-badge">VS</div>
        <div class="lobby-player" :class="{ waiting: !whitePlayer }">
          <span v-if="whitePlayer" class="big-avatar green-rabbit">兔</span>
          <span v-else class="big-avatar empty-avatar">?</span>
          <strong>{{ whitePlayer?.name || "等待加入" }}</strong>
          <small><i></i> {{ whitePlayer ? (whitePlayer.connected ? "已准备" : "连接中") : "邀请好友" }}</small>
        </div>
      </div>
      <div class="waiting-dots" aria-hidden="true"><i></i><i></i><i></i></div>
    </div>

    <div class="room-share-card">
      <div><span>房间号</span><strong>{{ roomCode || "------" }}</strong></div>
      <button type="button" @click="$emit('copy')">复制</button>
    </div>
    <button class="primary-cta" type="button" @click="$emit('share')">分享邀请链接</button>
    <button class="secondary-cta" type="button" @click="$emit('back')">返回首页，稍后再来</button>
    <div class="tiny-tip"><span>✦</span>好友打开链接即可自动加入，无需注册账号</div>
  </section>
</template>
