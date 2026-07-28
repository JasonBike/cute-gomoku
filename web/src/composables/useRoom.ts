import { computed, onBeforeUnmount, reactive } from "vue";
import type { ChatMessage, RoomCredentials, RoomState, ServerError } from "@/types/game";

const roomCodePattern = /^[A-HJ-NP-Z2-9]{6}$/;

export function useRoom(onNotice: (message: string, kind?: "default" | "chat") => void) {
  const state = reactive({
    roomCode: "",
    token: "",
    color: 0 as 0 | 1 | 2,
    socket: null as WebSocket | null,
    room: null as RoomState | null,
    connection: "idle" as "idle" | "connecting" | "online" | "offline",
    reconnectAttempts: 0,
    reconnectTimer: 0,
    manualClose: false,
  });

  const isMyTurn = computed(
    () =>
      state.room?.status === "playing" &&
      state.room.turn === state.color,
  );
  const opponent = computed(() =>
    state.room?.players.find((player) => player.color !== state.color),
  );
  const inviteURL = computed(() =>
    state.roomCode
      ? `${window.location.origin}/?room=${encodeURIComponent(state.roomCode)}`
      : `${window.location.origin}/`,
  );

  async function request<T>(path: string, options: RequestInit): Promise<T> {
    const response = await fetch(path, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
    });
    const body = (await response.json().catch(() => ({}))) as T & { message?: string };
    if (!response.ok) throw new Error(body.message || "服务暂时不可用");
    return body;
  }

  function setCredentials(credentials: RoomCredentials) {
    state.roomCode = credentials.roomCode;
    state.token = credentials.playerToken;
    state.color = credentials.color;
    state.manualClose = false;
    saveSession();
    syncURL(credentials.roomCode);
  }

  async function create(name = "小桃子") {
    const credentials = await request<RoomCredentials>("/api/rooms", {
      method: "POST",
      body: JSON.stringify({ name }),
    });
    setCredentials(credentials);
    connect();
  }

  async function join(code: string, name = "好友棋手", resumeExisting = true) {
    const normalized = code.trim().toUpperCase();
    if (!roomCodePattern.test(normalized)) throw new Error("请输入正确的六位房间号");
    const session = resumeExisting ? readSession(normalized) : null;
    const credentials = await request<RoomCredentials>(
      `/api/rooms/${encodeURIComponent(normalized)}/join`,
      {
        method: "POST",
        body: JSON.stringify({ name, playerToken: session?.token || "" }),
      },
    );
    setCredentials(credentials);
    connect();
  }

  async function restoreFromURL() {
    const code = (new URLSearchParams(window.location.search).get("room") || "")
      .trim()
      .toUpperCase();
    if (!roomCodePattern.test(code)) return false;

    state.roomCode = code;
    const session = readSession(code);
    if (session?.token && session.color) {
      state.token = session.token;
      state.color = session.color;
      saveSession();
      connect();
      return true;
    }

    await join(code);
    return true;
  }

  function connect() {
    if (!state.roomCode || !state.token) return;
    window.clearTimeout(state.reconnectTimer);
    if (state.socket) {
      state.socket.onclose = null;
      state.socket.close();
    }

    state.connection = "connecting";
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    const query = new URLSearchParams({
      room: state.roomCode,
      token: state.token,
    });
    const socket = new WebSocket(`${protocol}//${window.location.host}/ws?${query}`);
    state.socket = socket;

    socket.addEventListener("open", () => {
      state.connection = "online";
      state.reconnectAttempts = 0;
    });
    socket.addEventListener("message", (event) => {
      const message = JSON.parse(event.data) as RoomState | ServerError | ChatMessage;
      if (message.type === "error") {
        onNotice(message.message);
        return;
      }
      if (message.type === "chat") {
        if (message.from === state.color) {
          onNotice(`已发送：${message.text}`);
        } else {
          onNotice(`${message.name || "对手"}：${message.text}`, "chat");
          navigator.vibrate?.(60);
        }
        return;
      }
      message.moves = Array.isArray(message.moves) ? message.moves : [];
      message.players = Array.isArray(message.players) ? message.players : [];
      state.room = message;
    });
    socket.addEventListener("close", () => {
      if (state.manualClose || state.socket !== socket) return;
      state.connection = "offline";
      state.reconnectAttempts += 1;
      state.reconnectTimer = window.setTimeout(
        connect,
        Math.min(5000, 800 + state.reconnectAttempts * 500),
      );
    });
  }

  function send(message: Record<string, unknown>) {
    if (!state.socket || state.socket.readyState !== WebSocket.OPEN) {
      onNotice("正在重新连接，请稍等");
      return false;
    }
    state.socket.send(JSON.stringify(message));
    return true;
  }

  function move(row: number, column: number) {
    return send({ type: "move", row, column });
  }

  function resign() {
    return send({ type: "resign" });
  }

  function rematch() {
    return send({ type: "rematch" });
  }

  function requestUndo() {
    return send({ type: "undo_request" });
  }

  function respondUndo(accepted: boolean) {
    return send({ type: "undo_response", accepted });
  }

  function chat(text: string) {
    return send({ type: "chat", text });
  }

  function leave() {
    state.manualClose = true;
    window.clearTimeout(state.reconnectTimer);
    state.socket?.close();
    state.socket = null;
    state.room = null;
    state.roomCode = "";
    state.token = "";
    state.color = 0;
    state.connection = "idle";
    syncURL("");
  }

  function saveSession() {
    const value = JSON.stringify({ token: state.token, color: state.color });
    try {
      sessionStorage.setItem(`qiyu-room-${state.roomCode}`, value);
    } catch {
      // The current page connection still works when storage is unavailable.
    }
    try {
      localStorage.setItem(`qiyu-room-${state.roomCode}`, value);
    } catch {
      // The current page connection still works when storage is unavailable.
    }
  }

  function readSession(code: string): { token: string; color: 1 | 2 } | null {
    const key = `qiyu-room-${code}`;
    try {
      const session = JSON.parse(sessionStorage.getItem(key) || "null");
      if (session?.token && session?.color) return session;
    } catch {
      // Fall back to the persistent session below.
    }
    try {
      return JSON.parse(localStorage.getItem(key) || "null");
    } catch {
      return null;
    }
  }

  function syncURL(code: string) {
    const url = new URL(window.location.href);
    if (code) url.searchParams.set("room", code);
    else url.searchParams.delete("room");
    window.history.replaceState({}, "", `${url.pathname}${url.search}${url.hash}`);
  }

  onBeforeUnmount(() => {
    state.manualClose = true;
    window.clearTimeout(state.reconnectTimer);
    state.socket?.close();
  });

  return {
    state,
    isMyTurn,
    opponent,
    inviteURL,
    create,
    join,
    restoreFromURL,
    move,
    resign,
    rematch,
    requestUndo,
    respondUndo,
    chat,
    leave,
  };
}
