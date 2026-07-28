<script setup lang="ts">
import { computed, onMounted, ref, watch } from "vue";
import BottomNav, { type AppView } from "@/components/BottomNav.vue";
import { useRoom } from "@/composables/useRoom";
import GameView from "@/views/GameView.vue";
import HallView from "@/views/HallView.vue";
import HomeView from "@/views/HomeView.vue";
import LobbyView from "@/views/LobbyView.vue";
import ProfileView from "@/views/ProfileView.vue";
import RankingView from "@/views/RankingView.vue";

type View = AppView | "room" | "game";
type NoticeKind = "default" | "chat";

const currentView = ref<View>("home");
const joinOpen = ref(false);
const shareOpen = ref(false);
const resultOpen = ref(false);
const resignOpen = ref(false);
const exitOpen = ref(false);
const roomInput = ref("");
const toast = ref("");
const toastKind = ref<NoticeKind>("default");
const toastSerial = ref(0);
const leavingGame = ref(false);
const gameMinimized = ref(false);
let toastTimer = 0;

const room = useRoom(showNotice);
const players = computed(() => room.state.room?.players || []);
const isGameView = computed(() => currentView.value === "game");
const isMyTurn = room.isMyTurn;
const inviteURL = room.inviteURL;
const opponentOnline = computed(() => Boolean(room.opponent.value?.connected));
const undoRequestFromOpponent = computed(
  () =>
    Boolean(room.state.room?.undoRequester) &&
    room.state.room?.undoRequester !== room.state.color,
);
const resultWon = computed(
  () => room.state.room?.winner === room.state.color,
);
const resultTitle = computed(() => {
  if (room.state.room?.winner === 0) return "旗鼓相当，和棋啦";
  return resultWon.value ? "漂亮的五连！" : "差一点就赢了";
});
const resultDescription = computed(() => {
  const moves = room.state.room?.moves?.length || 0;
  if (room.state.room?.winner === 0) return `棋盘已经落满，本局共落下 ${moves} 手。`;
  return resultWon.value
    ? `你执${room.state.color === 1 ? "黑" : "白"}棋获胜，本局共落下 ${moves} 手。`
    : `对手完成五连，本局共落下 ${moves} 手。`;
});

function showNotice(message: string, kind: NoticeKind = "default") {
  toast.value = message;
  toastKind.value = kind;
  toastSerial.value += 1;
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(
    () => (toast.value = ""),
    kind === "chat" ? 3200 : 1900,
  );
}

async function createRoom() {
  gameMinimized.value = false;
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
    gameMinimized.value = false;
    await room.join(roomInput.value);
    joinOpen.value = false;
    currentView.value = "room";
  } catch (error) {
    showNotice(error instanceof Error ? error.message : "加入房间失败");
  }
}

async function joinHallRoom(roomCode: string) {
  roomInput.value = roomCode;
  try {
    gameMinimized.value = false;
    await room.join(roomCode, "好友棋手", false);
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
        url: inviteURL.value,
      });
      return;
    } catch (error) {
      if (error instanceof DOMException && error.name === "AbortError") return;
    }
  }
  await copy(inviteURL.value, "邀请链接已复制");
}

async function copy(value: string, message: string) {
  try {
    await navigator.clipboard.writeText(value);
    showNotice(message);
    return true;
  } catch {
    const input = document.createElement("textarea");
    input.value = value;
    input.style.position = "fixed";
    input.style.opacity = "0";
    document.body.appendChild(input);
    input.select();
    try {
      document.execCommand("copy");
      showNotice(message);
      return true;
    } catch {
      showNotice(`房间号：${room.state.roomCode}`);
      return false;
    } finally {
      input.remove();
    }
  }
}

async function copyInviteLink() {
  if (await copy(inviteURL.value, "邀请链接已复制")) {
    shareOpen.value = false;
  }
}

function navigate(view: AppView) {
  if (currentView.value === "room") {
    room.leave();
    shareOpen.value = false;
  }
  currentView.value = view;
}

function leaveRoom() {
  room.leave();
  shareOpen.value = false;
  resultOpen.value = false;
  resignOpen.value = false;
  exitOpen.value = false;
  gameMinimized.value = false;
  currentView.value = "home";
}

function returnToRoom() {
  gameMinimized.value = true;
  resultOpen.value = false;
  resignOpen.value = false;
  currentView.value = "room";
  document.title = "好友房间｜棋遇";
}

function enterGame() {
  gameMinimized.value = false;
  currentView.value = "game";
  resultOpen.value = false;
}

function requestLeaveRoom() {
  if (room.state.room?.status === "playing") {
    exitOpen.value = true;
    return;
  }
  leaveRoom();
}

function leaveGame(resignGame = false) {
  leavingGame.value = true;
  if (resignGame) room.resign();
  window.setTimeout(() => {
    room.leave();
    leavingGame.value = false;
  }, resignGame ? 80 : 0);
  exitOpen.value = false;
  resignOpen.value = false;
  resultOpen.value = false;
  gameMinimized.value = false;
  currentView.value = "home";
}

function confirmExitGame() {
  leaveGame(true);
}

function resign() {
  resignOpen.value = true;
}

function confirmResign() {
  if (room.resign()) resignOpen.value = false;
}

function requestUndo() {
  if (room.requestUndo()) showNotice("悔棋申请已发送，等待对手确认");
}

function respondUndo(accepted: boolean) {
  if (!room.respondUndo(accepted)) return;
  showNotice(accepted ? "已同意对手悔棋" : "已拒绝对手悔棋");
}

function requestRematch() {
  if (!opponentOnline.value) {
    showNotice("对手已经离开，重新连接后才能再来一局");
    return;
  }
  if (room.rematch()) {
    resultOpen.value = false;
    showNotice("已申请再来一局，等待对手确认");
  }
}

watch(
  () => room.state.room,
  (next, previous) => {
    if (!next) return;
    const wasWaitingForRematch = Boolean(
      previous?.players.find((player) => player.color === room.state.color)?.rematch,
    );
    const isWaitingForRematch = Boolean(
      next.players.find((player) => player.color === room.state.color)?.rematch,
    );
    if (
      next.status === "finished" &&
      wasWaitingForRematch &&
      !isWaitingForRematch
    ) {
      if (!gameMinimized.value) resultOpen.value = true;
      showNotice("再来一局申请已取消，请等对手在线后重新发起");
    }
    if (next.status === "waiting") {
      gameMinimized.value = false;
      currentView.value = "room";
    }
    if (next.status === "playing") {
      resultOpen.value = false;
      if (!gameMinimized.value) currentView.value = "game";
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
    if (
      next.status === "finished" &&
      previous?.status !== "finished" &&
      !leavingGame.value &&
      !gameMinimized.value
    ) {
      resignOpen.value = false;
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
    room.leave();
    currentView.value = "home";
  }
});
</script>

<template>
  <main class="app-shell" :class="{ 'game-active': isGameView, 'my-turn': isGameView && isMyTurn }">
    <HomeView
      v-if="currentView === 'home'"
      @create="createRoom"
      @join="joinOpen = true"
      @hall="currentView = 'hall'"
      @ranking="currentView = 'ranking'"
    />
    <LobbyView
      v-else-if="currentView === 'room'"
      :room-code="room.state.roomCode"
      :players="players"
      :connection="room.state.connection"
      :status="room.state.room?.status || 'waiting'"
      @back="requestLeaveRoom"
      @reenter="enterGame"
      @share="shareOpen = true"
      @copy="copy(room.state.roomCode, '房间号已复制')"
    />
    <GameView
      v-else-if="currentView === 'game' && room.state.room && room.state.color"
      :room="room.state.room"
      :my-color="room.state.color"
      :connection="room.state.connection"
      :is-my-turn="isMyTurn"
      @move="room.move"
      @exit="returnToRoom"
      @resign="resign"
      @undo="requestUndo"
      @rematch="requestRematch"
      @chat="room.chat"
      @notice="showNotice"
    />
    <HallView
      v-else-if="currentView === 'hall'"
      @create="createRoom"
      @join="joinHallRoom"
      @notice="showNotice"
    />
    <RankingView v-else-if="currentView === 'ranking'" />
    <ProfileView v-else-if="currentView === 'profile'" />

    <BottomNav
      v-if="!isGameView && currentView !== 'room'"
      :current="currentView === 'game' ? 'home' : currentView"
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
      <button class="copy-button" type="button" @click="copyInviteLink">复制邀请链接</button>
    </section>
  </div>

  <div v-if="resultOpen" class="sheet-backdrop result-backdrop">
    <section class="bottom-sheet result-sheet" role="dialog" aria-modal="true">
      <div class="result-face">{{ resultWon ? "• ᴗ •" : "• ︿ •" }}</div>
      <span class="section-kicker">{{ resultWon ? "FIVE IN A ROW!" : "GOOD GAME" }}</span>
      <h2>{{ resultTitle }}</h2>
      <p>{{ resultDescription }}</p>
      <div class="score-change"><span>对局类型</span><strong>好友局</strong><small>好友对战暂不计入排位积分</small></div>
      <button class="share-button" type="button" :disabled="!opponentOnline" @click="requestRematch">
        {{ opponentOnline ? "再来一局" : "对手离线，暂不能再来一局" }}
      </button>
      <button class="copy-button" type="button" @click="returnToRoom">返回房间</button>
    </section>
  </div>

  <div v-if="resignOpen" class="sheet-backdrop decision-backdrop" @click.self="resignOpen = false">
    <section class="bottom-sheet decision-sheet" role="dialog" aria-modal="true" aria-labelledby="resign-title">
      <button class="sheet-close" type="button" aria-label="取消认输" @click="resignOpen = false">×</button>
      <div class="decision-face resign-face">• ︿ •</div>
      <span class="section-kicker">RESIGN GAME</span>
      <h2 id="resign-title">真的要认输吗？</h2>
      <p>确认后本局立即结束，对手获得胜利。</p>
      <button class="decision-button danger" type="button" @click="confirmResign">确认认输</button>
      <button class="decision-button neutral" type="button" @click="resignOpen = false">继续下棋</button>
    </section>
  </div>

  <div v-if="exitOpen" class="sheet-backdrop decision-backdrop" @click.self="exitOpen = false">
    <section class="bottom-sheet decision-sheet" role="dialog" aria-modal="true" aria-labelledby="exit-title">
      <button class="sheet-close" type="button" aria-label="取消退出" @click="exitOpen = false">×</button>
      <div class="decision-face exit-face">↩</div>
      <span class="section-kicker">LEAVE GAME</span>
      <h2 id="exit-title">要离开这盘棋吗？</h2>
      <p>棋局还没有结束，现在退出会按认输处理。</p>
      <button class="decision-button danger" type="button" @click="confirmExitGame">退出并认输</button>
      <button class="decision-button neutral" type="button" @click="exitOpen = false">继续对局</button>
    </section>
  </div>

  <div v-if="undoRequestFromOpponent" class="sheet-backdrop decision-backdrop">
    <section class="bottom-sheet decision-sheet" role="dialog" aria-modal="true" aria-labelledby="undo-title">
      <div class="decision-face undo-face">↶</div>
      <span class="section-kicker">UNDO REQUEST</span>
      <h2 id="undo-title">对手申请悔棋</h2>
      <p>同意后会撤回对手最近一手，以及之后已经落下的棋子。</p>
      <button class="decision-button approve" type="button" @click="respondUndo(true)">同意悔棋</button>
      <button class="decision-button neutral" type="button" @click="respondUndo(false)">不同意，继续下</button>
    </section>
  </div>

  <div
    :key="toastSerial"
    class="toast"
    :class="{ show: toast, 'chat-toast': toastKind === 'chat' }"
    role="status"
    aria-live="polite"
  >
    <template v-if="toastKind === 'chat'">
      <span class="chat-toast-sparkle one" aria-hidden="true">✦</span>
      <span class="chat-toast-sparkle two" aria-hidden="true">✦</span>
      <svg class="chat-toast-avatar" viewBox="0 0 64 64" aria-hidden="true">
        <path d="M13 25 18 10l12 10M51 25 46 10 34 20" fill="#34302d" stroke="#34302d" stroke-width="3" stroke-linejoin="round" />
        <circle cx="32" cy="34" r="23" fill="#34302d" />
        <circle cx="25" cy="31" r="2.4" fill="#fffaf0" />
        <circle cx="39" cy="31" r="2.4" fill="#fffaf0" />
        <path d="M27 40c3.2 3.3 6.8 3.3 10 0" fill="none" stroke="#fffaf0" stroke-width="2.4" stroke-linecap="round" />
        <circle cx="18" cy="37" r="3" fill="#ff8a75" opacity=".75" />
        <circle cx="46" cy="37" r="3" fill="#ff8a75" opacity=".75" />
      </svg>
      <span class="chat-toast-copy">
        <small>好友发来消息</small>
        <strong>{{ toast }}</strong>
      </span>
      <i class="chat-toast-progress" aria-hidden="true"></i>
    </template>
    <template v-else>{{ toast }}</template>
  </div>
</template>
