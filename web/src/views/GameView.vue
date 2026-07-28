<script setup lang="ts">
import { computed, ref } from "vue";
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
  undo: [];
  rematch: [];
  chat: [message: string];
  notice: [message: string];
}>();

const chatOpen = ref(false);
const chatOptions = [
  { icon: "👋", text: "嗨，来一局！" },
  { icon: "👍", text: "好棋！" },
  { icon: "😮", text: "差一点！" },
  { icon: "🔥", text: "认真起来了" },
  { icon: "⏳", text: "慢慢想，不着急" },
  { icon: "🎉", text: "再来一局！" },
  { icon: "🏆", text: "你打的可太好了。" },
  { icon: "🌸", text: "我等的花儿都谢了" },
] as const;

const black = computed(() => props.room.players.find((player) => player.color === 1));
const white = computed(() => props.room.players.find((player) => player.color === 2));
const me = computed(() => props.room.players.find((player) => player.color === props.myColor));
const moves = computed(() => props.room.moves || []);
const waitingForRematch = computed(
  () => props.room.status === "finished" && Boolean(me.value?.rematch),
);
const turnText = computed(() => {
  if (waitingForRematch.value) return "等待对手同意再来一局";
  if (props.room.status === "finished") return "本局已经结束";
  return props.isMyTurn ? "轮到你落子" : "等待对手落子";
});
const turnDetail = computed(() => {
  if (waitingForRematch.value) return "对手确认后自动开局";
  if (props.room.status === "finished") return "可以邀请对手再下一盘";
  return `第 ${moves.value.length + 1} 手`;
});
const networkText = computed(() => {
  if (props.connection === "online") return "双方在线";
  if (props.connection === "offline") return "等待重连";
  return "连接中";
});

function sendChat(text: string) {
  emit("chat", text);
  chatOpen.value = false;
}
</script>

<template>
  <section class="view game-view active" aria-label="五子棋对局">
    <header class="game-topbar">
      <button class="icon-button" type="button" aria-label="返回好友房间" @click="emit('exit')">‹</button>
      <div class="game-mode"><strong>好友对战</strong><small><i></i> {{ networkText }}</small></div>
      <button class="icon-button" type="button" aria-label="音效" @click="emit('notice', '音效设置已保留')">♪</button>
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
      :class="{
        'my-turn': isMyTurn,
        'opponent-turn': !isMyTurn,
        'turn-pop': isMyTurn,
        'rematch-waiting': waitingForRematch,
      }"
      role="status"
      aria-live="polite"
    >
      <span class="turn-stone" :class="room.turn === 1 ? 'black' : 'white'"></span>
      <strong>{{ turnText }}</strong>
      <small>{{ turnDetail }}</small>
    </div>

    <div class="board-wrap">
      <GameBoard
        :board="room.board"
        :moves="moves"
        :winning-line="room.winningLine"
        :disabled="!isMyTurn || room.status !== 'playing' || Boolean(room.undoRequester)"
        @move="(row, column) => emit('move', row, column)"
        @occupied="emit('notice', '这里已经有棋子啦')"
      />
      <span class="board-label top">棋遇 · 好友局</span>
      <span class="board-label bottom">{{ room.roomCode }}</span>
    </div>

    <div class="game-actions-bar">
      <template v-if="room.status === 'playing'">
        <button
          type="button"
          :disabled="Boolean(room.undoRequester)"
          @click="emit('undo')"
        >
          ↶ {{ room.undoRequester === myColor ? "等待同意" : "悔棋" }}
        </button>
        <button type="button" @click="chatOpen = true"><span>☺</span>打招呼</button>
        <button type="button" @click="emit('resign')">⚑ 认输</button>
      </template>
      <template v-else>
        <button type="button" :disabled="waitingForRematch" @click="emit('rematch')">
          ↻ {{ waitingForRematch ? "等待同意" : "再来一局" }}
        </button>
        <button type="button" @click="chatOpen = true"><span>☺</span>打招呼</button>
        <button type="button" @click="emit('exit')">‹ 返回房间</button>
      </template>
    </div>
    <p class="game-tip">所有落子与胜负均由 Go 服务端校验</p>
  </section>

  <Teleport to="body">
    <div v-if="chatOpen" class="sheet-backdrop quick-chat-backdrop" @click.self="chatOpen = false">
      <section class="bottom-sheet quick-chat-sheet" role="dialog" aria-modal="true" aria-labelledby="quick-chat-title">
        <button class="sheet-close" type="button" aria-label="关闭快捷互动" @click="chatOpen = false">×</button>
        <div class="quick-chat-mascot" aria-hidden="true">
          <span>• ᴗ •</span>
          <i>✦</i>
        </div>
        <span class="section-kicker">QUICK CHAT</span>
        <h2 id="quick-chat-title">和对手说句话</h2>
        <p>选择一条快捷消息，对方会立即收到。</p>
        <div class="quick-chat-grid">
          <button
            v-for="option in chatOptions"
            :key="option.text"
            type="button"
            @click="sendChat(option.text)"
          >
            <span aria-hidden="true">{{ option.icon }}</span>
            <strong>{{ option.text }}</strong>
          </button>
        </div>
        <small class="quick-chat-tip">为了不打扰对局，每 2 秒最多发送一次</small>
      </section>
    </div>
  </Teleport>
</template>
