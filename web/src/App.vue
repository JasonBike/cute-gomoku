<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import BottomNav, { type AppView } from "@/components/BottomNav.vue";
import { useRoom } from "@/composables/useRoom";
import GameView from "@/views/GameView.vue";
import HomeView from "@/views/HomeView.vue";
import LobbyView from "@/views/LobbyView.vue";
import ProfileView from "@/views/ProfileView.vue";
import RankingView from "@/views/RankingView.vue";

type View = AppView | "room";

const currentView = ref<View>("home");
const joinOpen = ref(false);
const shareOpen = ref(false);
const resultOpen = ref(false);
const roomInput = ref("");
const toast = ref("");
let toastTimer = 0;

const room = useRoom(showNotice);
const players = computed(() => room.state.room?.players || []);
const isGameView = computed(() => currentView.value === "game");
const resultWon = computed(
  () => room.state.room?.winner === room.state.color,
);
const resultTitle = computed(() => {
  if (room.state.room?.winner === 0) return "旗鼓相当，和棋啦";
  return resultWon.value ? "漂亮的五连！" : "差一点就赢了";
});
const resultDescription = computed(() => {
  const moves = room.state.room?.moves.length || 0;
  if (room.state.room?.winner === 0) return `棋盘已经落满，本局共落下 ${moves} 手。`;
  return resultWon.value
    ? `你执${room.state.color === 1 ? "黑" : "白"}棋获胜，本局共落下 ${moves} 手。`
    : `对手完成五连，本局共落下 ${moves} 手。`;
});

function showNotice(message: string) {
  toast.value = message;
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => (toast.value = ""), 1900);
}

async function createRoom() {
  currentView.value = "room";
  try {
    await room.create();
  } catch (error) {
    showNotice(error instanceof Error ? error.message : "创建房间失败");
    currentView.value = "home";
  }
}

async function joinRoom() {
  try {
    await room.join(roomInput.value);
    joinOpen.value = false;
    currentView.value = "room";
  } catch (error) {
    showNotice(error instanceof Error ? error.message : "加入房间失败");
  }
}

async function shareRoom() {
  if (navigator.share) {
    try {
      await navigator.share({
        title: "来棋遇和我下一盘！",
        text: "我已经摆好棋盘啦，点开链接即可自动加入。",
        url: room.inviteURL.value,
      });
      return;
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
    }
  }
  await copy(room.inviteURL.value, "邀请链接已复制");
}

async function copy(value: string, message: string) {
  try {
    await navigator.clipboard.writeText(value);
    showNotice(message);
  } catch {
    showNotice(`房间号：${room.state.roomCode}`);
  }
}

function navigate(view: AppView) {
  if (view === "game") {
    showNotice("排位服务即将开放");
    return;
  }
  currentView.value = view;
}

function exitGame() {
  if (room.state.room?.status === "playing" && !window.confirm("棋局还没有结束，确定退出吗？")) return;
  if (room.state.room?.status === "playing") room.resign();
  window.setTimeout(room.leave, 80);
  resultOpen.value = false;
  currentView.value = "home";
}

function resign() {
  if (!window.confirm("确定认输这一局吗？")) return;
  room.resign();
}

function requestRematch() {
  if (room.rematch()) {
    resultOpen.value = false;
    showNotice("已申请再来一局，等待对手确认");
  }
}

watch(
  () => room.state.room,
  (next, previous) => {
    if (!next) return;
    if (next.status === "waiting") currentView.value = "room";
    if (next.status === "playing") {
      resultOpen.value = false;
      currentView.value = "game";
      const becameMyTurn =
        next.turn === room.state.color &&
        (previous?.status !== "playing" || previous.turn !== room.state.color);
      if (becameMyTurn) {
        document.title = "● 轮到你落子了｜棋遇";
        navigator.vibrate?.([70, 45, 70]);
      } else {
        document.title = "等待对手落子｜棋遇";
      }
    }
    if (next.status === "finished") {
      resultOpen.value = true;
      document.title = "棋局结束｜棋遇";
    }
  },
);

onMounted(async () => {
  const code = new URLSearchParams(window.location.search).get("room");
  if (!code) return;
  roomInput.value = code.toUpperCase();
  currentView.value = "room";
  try {
    await room.restoreFromURL();
  } catch (error) {
    showNotice(error instanceof Error ? error.message : "加入房间失败");
  }
});
</script>

<template>
  <main class="app-shell" :class="{ 'game-active': isGameView, 'my-turn': isGameView && room.isMyTurn.value }">
    <HomeView
      v-if="currentView === 'home'"
      @create="createRoom"
      @join="joinOpen = true"
      @ranked="showNotice('排位服务即将开放')"
      @ranking="currentView = 'ranking'"
    />
    <LobbyView
      v-else-if="currentView === 'room'"
      :room-code="room.state.roomCode"
      :players="players"
      :connection="room.state.connection"
      @back="currentView = 'home'"
      @share="shareOpen = true"
      @copy="copy(room.state.roomCode, '房间号已复制')"
    />
    <GameView
      v-else-if="currentView === 'game' && room.state.room && room.state.color"
      :room="room.state.room"
      :my-color="room.state.color"
      :connection="room.state.connection"
      :is-my-turn="room.isMyTurn.value"
      @move="room.move"
      @exit="exitGame"
      @resign="resign"
      @notice="showNotice"
    />
    <RankingView v-else-if="currentView === 'ranking'" />
    <ProfileView v-else-if="currentView === 'profile'" />

    <BottomNav
      v-if="!isGameView"
      :current="currentView === 'room' ? 'home' : currentView"
      @navigate="navigate"
    />
  </main>

  <div v-if="joinOpen" class="sheet-backdrop" @click.self="joinOpen = false">
    <section class="bottom-sheet join-sheet" role="dialog" aria-modal="true">
      <button class="sheet-close" type="button" aria-label="关闭" @click="joinOpen = false">×</button>
      <div class="join-sheet-icon">#</div>
      <span class="section-kicker">JOIN A ROOM</span>
      <h2>输入好友房间号</h2>
      <p>分享链接会自动加入，手动加入时在这里输入六位房间号。</p>
      <label class="room-input">
        <span>房间号</span>
        <input v-model.trim="roomInput" maxlength="6" placeholder="例如 7K2M8P" @keyup.enter="joinRoom" />
      </label>
      <button class="share-button" type="button" @click="joinRoom">加入房间</button>
    </section>
  </div>

  <div v-if="shareOpen" class="sheet-backdrop" @click.self="shareOpen = false">
    <section class="bottom-sheet" role="dialog" aria-modal="true">
      <button class="sheet-close" type="button" aria-label="关闭" @click="shareOpen = false">×</button>
      <div class="sheet-illustration"><span class="mini-stone black"></span><span class="link-line"></span><span class="mini-stone white"></span></div>
      <span class="section-kicker">好友房间已创建</span>
      <h2>邀请朋友来下一盘</h2>
      <p>朋友打开链接后，会直接进入你的房间。</p>
      <div class="room-code"><span>房间号</span><strong>{{ room.state.roomCode }}</strong></div>
      <button class="share-button" type="button" @click="shareRoom">分享给好友</button>
      <button class="copy-button" type="button" @click="copy(room.inviteURL.value, '邀请链接已复制')">复制邀请链接</button>
    </section>
  </div>

  <div v-if="resultOpen" class="sheet-backdrop result-backdrop">
    <section class="bottom-sheet result-sheet" role="dialog" aria-modal="true">
      <div class="result-face">{{ resultWon ? "• ᴗ •" : "• ︿ •" }}</div>
      <span class="section-kicker">{{ resultWon ? "FIVE IN A ROW!" : "GOOD GAME" }}</span>
      <h2>{{ resultTitle }}</h2>
      <p>{{ resultDescription }}</p>
      <div class="score-change"><span>对局类型</span><strong>好友局</strong><small>好友对战暂不计入排位积分</small></div>
      <button class="share-button" type="button" @click="requestRematch">再来一局</button>
      <button class="copy-button" type="button" @click="room.leave(); resultOpen = false; currentView = 'home'">返回首页</button>
    </section>
  </div>

  <div class="toast" :class="{ show: toast }" role="status">{{ toast }}</div>
</template>
