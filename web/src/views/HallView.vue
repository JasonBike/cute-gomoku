<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from "vue";
import type { HallRoom } from "@/types/game";

const emit = defineEmits<{
  create: [];
  join: [roomCode: string];
  notice: [message: string];
}>();

const rooms = ref<HallRoom[]>([]);
const filter = ref<"joinable" | "all">("joinable");
const refreshing = ref(false);
const loaded = ref(false);
let refreshTimer = 0;

const joinableRooms = computed(() => rooms.value.filter((room) => room.joinable));
const visibleRooms = computed(() =>
  filter.value === "joinable" ? joinableRooms.value : rooms.value,
);

async function refreshRooms(showFeedback = false) {
  if (refreshing.value) return;
  refreshing.value = true;
  try {
    const response = await fetch("/api/rooms", { cache: "no-store" });
    if (!response.ok) throw new Error("大厅暂时走丢了");
    const result = (await response.json()) as { rooms?: HallRoom[] };
    rooms.value = Array.isArray(result.rooms) ? result.rooms : [];
    if (showFeedback) emit("notice", "大厅已经刷新");
  } catch (error) {
    emit("notice", error instanceof Error ? error.message : "大厅刷新失败");
  } finally {
    loaded.value = true;
    refreshing.value = false;
  }
}

function relativeTime(createdAt: number) {
  const minutes = Math.max(0, Math.floor((Date.now() - createdAt) / 60000));
  if (minutes < 1) return "刚刚创建";
  if (minutes < 60) return `${minutes} 分钟前`;
  const hours = Math.floor(minutes / 60);
  return `${hours} 小时前`;
}

function roomStatus(room: HallRoom) {
  if (room.joinable) return "等你入座";
  if (room.status === "playing") return `进行到第 ${room.moveCount + 1} 手`;
  return "本局已结束";
}

onMounted(() => {
  void refreshRooms();
  refreshTimer = window.setInterval(() => void refreshRooms(), 5000);
});

onBeforeUnmount(() => window.clearInterval(refreshTimer));
</script>

<template>
  <section class="view hall-view active" aria-label="在线大厅">
    <header class="hall-header">
      <div>
        <span class="section-kicker">GOMOKU LOUNGE</span>
        <h1>棋友大厅</h1>
        <p><i></i>{{ rooms.length }} 个房间正在热闹中</p>
      </div>
      <button
        class="hall-refresh"
        :class="{ spinning: refreshing }"
        type="button"
        aria-label="刷新大厅"
        @click="refreshRooms(true)"
      >
        ↻
      </button>
    </header>

    <section class="hall-hero">
      <div class="hall-hero-copy">
        <span>现在有 {{ joinableRooms.length }} 张空椅子</span>
        <h2>挑个顺眼的棋友，<br />坐下来下一盘吧！</h2>
        <button type="button" @click="emit('create')">＋ 我也开一桌</button>
      </div>
      <div class="hall-mascots" aria-hidden="true">
        <span class="hall-sparkle one">✦</span>
        <span class="hall-sparkle two">✦</span>
        <div class="hall-stone black"><i></i><b>ᴗ</b></div>
        <div class="hall-stone white"><i></i><b>ᴗ</b></div>
        <small>来嘛！</small>
      </div>
    </section>

    <div class="hall-toolbar">
      <div class="hall-tabs" role="tablist" aria-label="房间筛选">
        <button
          :class="{ active: filter === 'joinable' }"
          type="button"
          role="tab"
          @click="filter = 'joinable'"
        >
          可加入 <span>{{ joinableRooms.length }}</span>
        </button>
        <button
          :class="{ active: filter === 'all' }"
          type="button"
          role="tab"
          @click="filter = 'all'"
        >
          全部房间 <span>{{ rooms.length }}</span>
        </button>
      </div>
      <small>每 5 秒自动刷新</small>
    </div>

    <div v-if="!loaded" class="hall-loading" aria-label="正在加载房间">
      <i></i><i></i><i></i>
    </div>

    <div v-else-if="visibleRooms.length" class="hall-room-list">
      <article
        v-for="(room, index) in visibleRooms"
        :key="room.roomCode"
        class="hall-room-card"
        :class="{ joinable: room.joinable }"
      >
        <div class="hall-card-head">
          <span class="hall-host-avatar" :class="`avatar-${(index % 3) + 1}`">
            <i></i><b>{{ room.hostName.slice(0, 1) }}</b>
          </span>
          <span class="hall-host-copy">
            <small>房主</small>
            <strong>{{ room.hostName }}</strong>
            <em>{{ relativeTime(room.createdAt) }}</em>
          </span>
          <span class="hall-status-pill" :class="{ live: room.joinable }">
            <i></i>{{ roomStatus(room) }}
          </span>
        </div>

        <div class="hall-card-body">
          <div class="hall-seats" aria-label="房间座位">
            <span class="seat occupied black">●</span>
            <span class="seat" :class="{ occupied: room.playerCount > 1 }">
              {{ room.playerCount > 1 ? "○" : "＋" }}
            </span>
            <small>{{ room.playerCount }}/2 人</small>
          </div>
          <div class="hall-room-code">
            <small>ROOM</small>
            <strong>{{ room.roomCode }}</strong>
          </div>
          <button
            type="button"
            :disabled="!room.joinable"
            @click="emit('join', room.roomCode)"
          >
            {{ room.joinable ? "加入对局" : room.status === "playing" ? "对局中" : "已结束" }}
            <span v-if="room.joinable">›</span>
          </button>
        </div>
      </article>
    </div>

    <div v-else class="hall-empty">
      <div class="empty-board" aria-hidden="true">
        <i class="empty-stone">• ︿ •</i>
        <span>✦</span>
      </div>
      <h2>{{ filter === "joinable" ? "暂时没有空桌" : "大厅现在静悄悄" }}</h2>
      <p>不如你先摆好棋盘，等第一位棋友来敲门。</p>
      <button type="button" @click="emit('create')">创建第一个房间</button>
    </div>
  </section>
</template>
