<script setup lang="ts">
import { computed } from "vue";
import GameBoard from "@/components/GameBoard.vue";
import type { RoomState } from "@/types/game";

const props = defineProps<{
  room: RoomState;
  myColor: 1 | 2;
  connection: "idle" | "connecting" | "online" | "offline";
  isMyTurn: boolean;
}>();

const emit = defineEmits<{
  move: [row: number, column: number];
  exit: [];
  resign: [];
  notice: [message: string];
}>();

const black = computed(() => props.room.players.find((player) => player.color === 1));
const white = computed(() => props.room.players.find((player) => player.color === 2));
const turnText = computed(() => (props.isMyTurn ? "轮到你落子" : "等待对手落子"));
const networkText = computed(() => {
  if (props.connection === "online") return "双方在线";
  if (props.connection === "offline") return "等待重连";
  return "连接中";
});
</script>

<template>
  <section class="view game-view active" aria-label="五子棋对局">
    <header class="game-topbar">
      <button class="icon-button" type="button" aria-label="退出对局" @click="$emit('exit')">‹</button>
      <div class="game-mode"><strong>好友对战</strong><small><i></i> {{ networkText }}</small></div>
      <button class="icon-button" type="button" aria-label="音效" @click="$emit('notice', '音效设置已保留')">♪</button>
    </header>

    <div class="players-strip">
      <div class="player-panel" :class="{ 'active-player': room.turn === 1 }">
        <span class="avatar avatar-small"><span class="avatar-face">•ᴗ•</span></span>
        <span><strong>{{ black?.name || "黑棋" }}</strong><small>黑棋 · {{ myColor === 1 ? "我" : "对手" }}</small></span>
        <b class="player-time">{{ black?.connected ? "在线" : "离线" }}</b>
      </div>
      <div class="player-panel" :class="{ 'active-player': room.turn === 2 }">
        <span class="avatar avatar-yellow">团</span>
        <span><strong>{{ white?.name || "白棋" }}</strong><small>白棋 · {{ myColor === 2 ? "我" : "对手" }}</small></span>
        <b class="player-time">{{ white?.connected ? "在线" : "离线" }}</b>
      </div>
    </div>

    <div
      class="turn-banner"
      :class="{ 'my-turn': isMyTurn, 'opponent-turn': !isMyTurn, 'turn-pop': isMyTurn }"
      role="status"
      aria-live="polite"
    >
      <span class="turn-stone" :class="room.turn === 1 ? 'black' : 'white'"></span>
      <strong>{{ turnText }}</strong>
      <small>第 {{ room.moves.length + 1 }} 手</small>
    </div>

    <div class="board-wrap">
      <GameBoard
        :board="room.board"
        :moves="room.moves"
        :winning-line="room.winningLine"
        :disabled="!isMyTurn || room.status !== 'playing'"
        @move="(row, column) => emit('move', row, column)"
        @occupied="$emit('notice', '这里已经有棋子啦')"
      />
      <span class="board-label top">棋遇 · 好友局</span>
      <span class="board-label bottom">{{ room.roomCode }}</span>
    </div>

    <div class="game-actions-bar">
      <button type="button" @click="$emit('notice', '联机对局暂不支持悔棋')">↶ 悔棋</button>
      <button type="button" @click="$emit('notice', '你向对手发送了：好棋！')"><span>☺</span>打招呼</button>
      <button type="button" @click="$emit('resign')">⚑ 认输</button>
    </div>
    <p class="game-tip">所有落子与胜负均由 Go 服务端校验</p>
  </section>
</template>
