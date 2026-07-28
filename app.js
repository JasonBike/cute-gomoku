const appShell = document.querySelector(".app-shell");
const views = [...document.querySelectorAll(".view")];
const navItems = [...document.querySelectorAll(".nav-item")];
const sheet = document.querySelector("#sheetBackdrop");
const joinBackdrop = document.querySelector("#joinBackdrop");
const resultBackdrop = document.querySelector("#resultBackdrop");
const toast = document.querySelector("#toast");
const canvas = document.querySelector("#gomokuBoard");
const context = canvas.getContext("2d");

const BOARD_SIZE = 15;
const BOARD_PADDING = 42;
const BOARD_LENGTH = canvas.width - BOARD_PADDING * 2;
const CELL_SIZE = BOARD_LENGTH / (BOARD_SIZE - 1);
let roomLink = window.location.href.split("?")[0];

let toastTimer;
let gameTimer;
let soundEnabled = true;
let resultShownForMoveCount = -1;

const online = {
  socket: null,
  roomCode: "",
  token: "",
  color: 0,
  reconnectTimer: 0,
  reconnectAttempts: 0,
  manualClose: false,
};

const playerStats = readStats();

const game = {
  board: createEmptyBoard(),
  moves: [],
  current: 1,
  ended: false,
  mode: "local",
  blackSeconds: 600,
  whiteSeconds: 600,
  winningLine: null,
};

function createEmptyBoard() {
  return Array.from({ length: BOARD_SIZE }, () => Array(BOARD_SIZE).fill(0));
}

function readStats() {
  const fallback = { games: 36, wins: 22, score: 806 };

  try {
    return { ...fallback, ...JSON.parse(localStorage.getItem("qiyu-player") || "{}") };
  } catch {
    return fallback;
  }
}

function saveStats() {
  try {
    localStorage.setItem("qiyu-player", JSON.stringify(playerStats));
  } catch {
    // The visual prototype still works when local storage is unavailable.
  }
}

function updateProfileStats() {
  const rate = playerStats.games ? Math.round((playerStats.wins / playerStats.games) * 100) : 0;
  document.querySelector("#profileGames").textContent = playerStats.games;
  document.querySelector("#profileWinRate").textContent = `${rate}%`;
  document.querySelector("#profileScore").textContent = playerStats.score;
}

function showView(name) {
  views.forEach((view) => view.classList.toggle("active", view.dataset.view === name));
  navItems.forEach((item) => item.classList.toggle("active", item.dataset.target === name));
  appShell.classList.toggle("game-active", name === "game");
  window.scrollTo({ top: 0, behavior: "smooth" });

  if (name === "game") {
    requestAnimationFrame(drawBoard);
    startGameTimer();
  } else {
    stopGameTimer();
  }
}

function showToast(message) {
  toast.textContent = message;
  toast.classList.add("show");
  window.clearTimeout(toastTimer);
  toastTimer = window.setTimeout(() => toast.classList.remove("show"), 1800);
}

function openBackdrop(element) {
  element.hidden = false;
  document.body.style.overflow = "hidden";
}

function closeBackdrop(element) {
  element.hidden = true;
  document.body.style.overflow = "";
}

function copyText(value, successMessage) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(value).then(
      () => showToast(successMessage),
      () => fallbackCopy(value, successMessage),
    );
    return;
  }

  fallbackCopy(value, successMessage);
}

function fallbackCopy(value, successMessage) {
  const input = document.createElement("textarea");
  input.value = value;
  input.style.position = "fixed";
  input.style.opacity = "0";
  document.body.appendChild(input);
  input.select();

  try {
    document.execCommand("copy");
    showToast(successMessage);
  } catch {
    showToast(online.roomCode ? `房间号：${online.roomCode}` : "复制失败，请手动输入房间号");
  } finally {
    input.remove();
  }
}

async function shareRoom() {
  if (navigator.share) {
    try {
      await navigator.share({
        title: "来棋遇和我下一盘！",
        text: "我已经摆好棋盘啦，点开链接就能加入。",
        url: roomLink,
      });
      return;
    } catch (error) {
      if (error.name === "AbortError") return;
    }
  }

  copyText(roomLink, "邀请链接已复制");
}

function resetLobby() {
  document.querySelector("#waitingPlayer").classList.add("waiting");
  document.querySelector("#waitingPlayer .big-avatar").className = "big-avatar empty-avatar";
  document.querySelector("#waitingPlayer .big-avatar").textContent = "?";
  document.querySelector("#opponentName").textContent = "等待加入";
  document.querySelector("#opponentStatus").innerHTML = "<i></i> 邀请好友";
  document.querySelector("#lobbyMessage").textContent = "正在等待朋友从邀请链接进入…";
  document.querySelector("#waitingDots").hidden = false;
}

async function apiRequest(path, options = {}) {
  const response = await fetch(path, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options.headers,
    },
  });
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.message || "服务暂时不可用");
  return body;
}

function updateRoomCode(code) {
  online.roomCode = code;
  roomLink = `${window.location.origin}/?room=${encodeURIComponent(code)}`;
  document.querySelectorAll("[data-room-code]").forEach((element) => {
    element.textContent = code || "------";
  });
}

function saveRoomSession() {
  try {
    localStorage.setItem(
      `qiyu-room-${online.roomCode}`,
      JSON.stringify({ token: online.token, color: online.color }),
    );
  } catch {
    // Reconnection still works until the current page is refreshed.
  }
}

function readRoomSession(code) {
  try {
    return JSON.parse(localStorage.getItem(`qiyu-room-${code}`) || "null");
  } catch {
    return null;
  }
}

async function createOnlineRoom() {
  if (window.location.protocol === "file:") {
    showToast("请通过 Go 服务打开页面后再创建房间");
    return;
  }
  try {
    const created = await apiRequest("/api/rooms", {
      method: "POST",
      body: JSON.stringify({ name: "小桃子" }),
    });
    online.token = created.playerToken;
    online.color = created.color;
    online.manualClose = false;
    updateRoomCode(created.roomCode);
    saveRoomSession();
    resetLobby();
    showView("room");
    connectRoomSocket();
  } catch (error) {
    showToast(error.message);
  }
}

async function joinOnlineRoom(code) {
  if (window.location.protocol === "file:") {
    showToast("请通过 Go 服务打开页面后再加入房间");
    return;
  }
  try {
    const joined = await apiRequest(`/api/rooms/${encodeURIComponent(code)}/join`, {
      method: "POST",
      body: JSON.stringify({ name: "好友棋手" }),
    });
    online.token = joined.playerToken;
    online.color = joined.color;
    online.manualClose = false;
    updateRoomCode(joined.roomCode);
    saveRoomSession();
    closeBackdrop(joinBackdrop);
    resetLobby();
    showView("room");
    connectRoomSocket();
  } catch (error) {
    showToast(error.message);
  }
}

function connectRoomSocket() {
  if (!online.roomCode || !online.token) return;
  window.clearTimeout(online.reconnectTimer);
  if (online.socket) {
    online.socket.onclose = null;
    online.socket.close();
  }

  const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
  const query = new URLSearchParams({ room: online.roomCode, token: online.token });
  const socket = new WebSocket(`${protocol}//${window.location.host}/ws?${query}`);
  online.socket = socket;

  socket.addEventListener("open", () => {
    online.reconnectAttempts = 0;
    document.querySelector("#lobbyMessage").textContent = "连接成功，正在等待好友加入…";
  });
  socket.addEventListener("message", (event) => {
    const message = JSON.parse(event.data);
    if (message.type === "error") {
      showToast(message.message);
      return;
    }
    if (message.type === "state") handleRoomState(message);
  });
  socket.addEventListener("close", () => {
    if (online.manualClose || online.socket !== socket) return;
    document.querySelector("#lobbyMessage").textContent = "连接中断，正在自动重连…";
    online.reconnectAttempts += 1;
    online.reconnectTimer = window.setTimeout(
      connectRoomSocket,
      Math.min(5000, 800 + online.reconnectAttempts * 500),
    );
  });
}

function handleRoomState(state) {
  updateRoomCode(state.roomCode);
  game.mode = "online";
  game.board = state.board;
  game.moves = state.moves || [];
  game.current = state.turn;
  game.winningLine = state.winningLine?.length === 2 ? state.winningLine : null;
  game.ended = state.status === "finished";
  syncPlayerUI(state.players);
  updateTurnUI();
  drawBoard();

  if (state.status === "waiting") {
    resetLobby();
    syncLobbyPlayers(state.players);
    if (!document.querySelector('[data-view="room"]').classList.contains("active")) showView("room");
    return;
  }

  if (state.status === "playing") {
    resultShownForMoveCount = -1;
    closeBackdrop(resultBackdrop);
    if (!document.querySelector('[data-view="game"]').classList.contains("active")) showView("game");
    return;
  }

  if (state.status === "finished" && resultShownForMoveCount !== state.moves.length) {
    resultShownForMoveCount = state.moves.length;
    window.setTimeout(() => showResult(state.winner), 450);
  }
}

function syncLobbyPlayers(players) {
  const blackPlayer = players.find((player) => player.color === 1);
  const whitePlayer = players.find((player) => player.color === 2);
  if (blackPlayer) {
    document.querySelector(".lobby-player.ready strong").textContent = blackPlayer.name;
  }
  if (!whitePlayer) return;

  const waitingPlayer = document.querySelector("#waitingPlayer");
  waitingPlayer.classList.remove("waiting");
  waitingPlayer.querySelector(".big-avatar").className = "big-avatar green-rabbit";
  waitingPlayer.querySelector(".big-avatar").textContent = "兔";
  document.querySelector("#opponentName").textContent = whitePlayer.name;
  document.querySelector("#opponentStatus").innerHTML = `<i></i> ${whitePlayer.connected ? "已准备" : "连接中"}`;
  document.querySelector("#lobbyMessage").textContent = "好友已经进入，正在准备开局…";
  document.querySelector("#waitingDots").hidden = false;
}

function syncPlayerUI(players) {
  const blackPlayer = players.find((player) => player.color === 1);
  const whitePlayer = players.find((player) => player.color === 2);
  if (blackPlayer) document.querySelector("#blackPlayerName").textContent = blackPlayer.name;
  if (whitePlayer) document.querySelector("#gameOpponentName").textContent = whitePlayer.name;

  const opponent = players.find((player) => player.color !== online.color);
  const connectionLabel = document.querySelector(".game-mode small");
  if (opponent?.connected) {
    connectionLabel.innerHTML = "<i></i> 双方在线";
  } else {
    connectionLabel.innerHTML = "<i></i> 等待对手重连";
  }
}

function sendRoomAction(message) {
  if (!online.socket || online.socket.readyState !== WebSocket.OPEN) {
    showToast("正在重新连接，请稍等");
    return false;
  }
  online.socket.send(JSON.stringify(message));
  return true;
}

function leaveOnlineRoom() {
  online.manualClose = true;
  window.clearTimeout(online.reconnectTimer);
  if (online.socket) online.socket.close();
  online.socket = null;
  online.roomCode = "";
  online.token = "";
  online.color = 0;
  updateRoomCode("");
}

function startGame(mode, opponent = "糯米团") {
  game.mode = mode;
  document.querySelector("#gameModeTitle").textContent = mode === "ranked" ? "排位对战" : "好友对战";
  document.querySelector("#gameOpponentName").textContent = opponent;
  resetGame();
  showView("game");
}

function resetGame() {
  game.board = createEmptyBoard();
  game.moves = [];
  game.current = 1;
  game.ended = false;
  game.blackSeconds = 600;
  game.whiteSeconds = 600;
  game.winningLine = null;
  closeBackdrop(resultBackdrop);
  updateTurnUI();
  updateTimers();
  drawBoard();
}

function drawBoard() {
  const width = canvas.width;
  context.clearRect(0, 0, width, width);

  const paperGradient = context.createLinearGradient(0, 0, width, width);
  paperGradient.addColorStop(0, "#f3ca89");
  paperGradient.addColorStop(1, "#e6b26a");
  context.fillStyle = paperGradient;
  context.fillRect(0, 0, width, width);

  context.strokeStyle = "rgba(82, 56, 35, 0.78)";
  context.lineWidth = 2;

  for (let index = 0; index < BOARD_SIZE; index += 1) {
    const position = BOARD_PADDING + CELL_SIZE * index;
    context.beginPath();
    context.moveTo(BOARD_PADDING, position);
    context.lineTo(width - BOARD_PADDING, position);
    context.stroke();
    context.beginPath();
    context.moveTo(position, BOARD_PADDING);
    context.lineTo(position, width - BOARD_PADDING);
    context.stroke();
  }

  [3, 7, 11].forEach((row) => {
    [3, 7, 11].forEach((column) => {
      context.beginPath();
      context.arc(
        BOARD_PADDING + column * CELL_SIZE,
        BOARD_PADDING + row * CELL_SIZE,
        row === 7 && column === 7 ? 6 : 4.5,
        0,
        Math.PI * 2,
      );
      context.fillStyle = "rgba(77, 51, 31, 0.82)";
      context.fill();
    });
  });

  game.moves.forEach((move, index) => drawStone(move.row, move.column, move.player, index === game.moves.length - 1));

  if (game.winningLine) {
    const [start, end] = game.winningLine;
    context.beginPath();
    context.moveTo(BOARD_PADDING + start.column * CELL_SIZE, BOARD_PADDING + start.row * CELL_SIZE);
    context.lineTo(BOARD_PADDING + end.column * CELL_SIZE, BOARD_PADDING + end.row * CELL_SIZE);
    context.strokeStyle = "#ff795f";
    context.lineWidth = 9;
    context.lineCap = "round";
    context.globalAlpha = 0.82;
    context.stroke();
    context.globalAlpha = 1;
  }
}

function drawStone(row, column, player, isLast) {
  const x = BOARD_PADDING + column * CELL_SIZE;
  const y = BOARD_PADDING + row * CELL_SIZE;
  const radius = CELL_SIZE * 0.39;
  const gradient = context.createRadialGradient(x - radius * 0.35, y - radius * 0.4, 2, x, y, radius);

  if (player === 1) {
    gradient.addColorStop(0, "#625a55");
    gradient.addColorStop(0.52, "#302b28");
    gradient.addColorStop(1, "#171514");
  } else {
    gradient.addColorStop(0, "#ffffff");
    gradient.addColorStop(0.68, "#f5f0e8");
    gradient.addColorStop(1, "#d8cfc4");
  }

  context.save();
  context.shadowColor = "rgba(63, 43, 29, 0.28)";
  context.shadowBlur = 8;
  context.shadowOffsetY = 4;
  context.beginPath();
  context.arc(x, y, radius, 0, Math.PI * 2);
  context.fillStyle = gradient;
  context.fill();
  context.restore();

  context.beginPath();
  context.arc(x, y, radius, 0, Math.PI * 2);
  context.strokeStyle = player === 1 ? "#161412" : "#8f8479";
  context.lineWidth = 1.5;
  context.stroke();

  if (isLast) {
    context.beginPath();
    context.arc(x, y, 5, 0, Math.PI * 2);
    context.fillStyle = "#ff795f";
    context.fill();
  }
}

function handleBoardPointer(event) {
  if (game.ended) return;

  const bounds = canvas.getBoundingClientRect();
  const scaleX = canvas.width / bounds.width;
  const scaleY = canvas.height / bounds.height;
  const x = (event.clientX - bounds.left) * scaleX;
  const y = (event.clientY - bounds.top) * scaleY;
  const column = Math.round((x - BOARD_PADDING) / CELL_SIZE);
  const row = Math.round((y - BOARD_PADDING) / CELL_SIZE);

  if (row < 0 || row >= BOARD_SIZE || column < 0 || column >= BOARD_SIZE) return;
  if (game.board[row][column] !== 0) {
    showToast("这里已经有棋子啦");
    return;
  }

  if (game.mode === "online") {
    if (game.current !== online.color) {
      showToast("还没有轮到你");
      return;
    }
    sendRoomAction({ type: "move", row, column });
    return;
  }

  placeStone(row, column);
}

function placeStone(row, column) {
  const player = game.current;
  game.board[row][column] = player;
  game.moves.push({ row, column, player });
  game.winningLine = findWinningLine(row, column, player);
  drawBoard();

  if (game.winningLine) {
    game.ended = true;
    window.setTimeout(() => showResult(player), 550);
    return;
  }

  if (game.moves.length === BOARD_SIZE * BOARD_SIZE) {
    game.ended = true;
    window.setTimeout(() => showResult(0), 350);
    return;
  }

  game.current = player === 1 ? 2 : 1;
  updateTurnUI();
}

function findWinningLine(row, column, player) {
  const directions = [
    [1, 0],
    [0, 1],
    [1, 1],
    [1, -1],
  ];

  for (const [rowStep, columnStep] of directions) {
    const stones = [{ row, column }];

    for (const sign of [-1, 1]) {
      let nextRow = row + rowStep * sign;
      let nextColumn = column + columnStep * sign;

      while (
        nextRow >= 0 &&
        nextRow < BOARD_SIZE &&
        nextColumn >= 0 &&
        nextColumn < BOARD_SIZE &&
        game.board[nextRow][nextColumn] === player
      ) {
        sign === -1 ? stones.unshift({ row: nextRow, column: nextColumn }) : stones.push({ row: nextRow, column: nextColumn });
        nextRow += rowStep * sign;
        nextColumn += columnStep * sign;
      }
    }

    if (stones.length >= 5) return [stones[0], stones[stones.length - 1]];
  }

  return null;
}

function updateTurnUI() {
  const isBlack = game.current === 1;
  const isMyTurn = game.mode !== "online" || game.current === online.color;
  const banner = document.querySelector("#turnBanner");
  banner.querySelector(".turn-stone").className = `turn-stone ${isBlack ? "black" : "white"}`;
  banner.querySelector("strong").textContent =
    game.mode === "online"
      ? isMyTurn
        ? "轮到你落子"
        : "等待对手落子"
      : isBlack
        ? "轮到黑棋落子"
        : "轮到白棋落子";
  banner.querySelector("small").textContent = `${game.moves.length + 1} 手`;
  document.querySelector("#blackPlayerPanel").classList.toggle("active-player", isBlack);
  document.querySelector("#whitePlayerPanel").classList.toggle("active-player", !isBlack);
}

function startGameTimer() {
  stopGameTimer();
  gameTimer = window.setInterval(() => {
    if (game.ended) return;
    if (game.current === 1) game.blackSeconds = Math.max(0, game.blackSeconds - 1);
    else game.whiteSeconds = Math.max(0, game.whiteSeconds - 1);
    updateTimers();
  }, 1000);
}

function stopGameTimer() {
  window.clearInterval(gameTimer);
}

function updateTimers() {
  document.querySelector("#blackTime").textContent = formatTime(game.blackSeconds);
  document.querySelector("#whiteTime").textContent = formatTime(game.whiteSeconds);
}

function formatTime(seconds) {
  const minutes = Math.floor(seconds / 60).toString().padStart(2, "0");
  const remainder = (seconds % 60).toString().padStart(2, "0");
  return `${minutes}:${remainder}`;
}

function undoMove() {
  if (game.mode === "online") {
    showToast("联机对局暂不支持悔棋");
    return;
  }
  if (!game.moves.length || game.ended) {
    showToast("现在还不能悔棋");
    return;
  }

  const move = game.moves.pop();
  game.board[move.row][move.column] = 0;
  game.current = move.player;
  game.winningLine = null;
  updateTurnUI();
  drawBoard();
  showToast("已撤回上一步");
}

function showResult(winner, resigned = false) {
  game.ended = true;
  stopGameTimer();
  const isWin = winner === (game.mode === "online" ? online.color : 1);
  const isDraw = winner === 0;
  const title = document.querySelector("#resultTitle");
  const kicker = document.querySelector("#resultKicker");
  const description = document.querySelector("#resultDescription");
  const face = document.querySelector("#resultFace");
  const scoreChange = document.querySelector("#scoreChange");
  const scoreDetail = document.querySelector(".score-change small");

  if (isDraw) {
    kicker.textContent = "A CLOSE GAME";
    title.textContent = "旗鼓相当，和棋啦";
    description.textContent = `棋盘已经落满，本局共落下 ${game.moves.length} 手。`;
    face.textContent = "• ︿ •";
    scoreChange.textContent = "+2";
  } else if (isWin) {
    kicker.textContent = "FIVE IN A ROW!";
    title.textContent = "漂亮的五连！";
    description.textContent = `你执${online.color === 2 ? "白" : "黑"}棋获胜，本局共落下 ${game.moves.length} 手。`;
    face.textContent = "• ᴗ •";
    scoreChange.textContent = "+25";
  } else {
    kicker.textContent = resigned ? "GOOD GAME" : "ALMOST THERE";
    title.textContent = resigned ? "这局先到这里" : "差一点就赢了";
    description.textContent = resigned ? "你已认输，整理思路再来一盘吧。" : `对手完成五连，本局共落下 ${game.moves.length} 手。`;
    face.textContent = "• ︿ •";
    scoreChange.textContent = "-12";
  }

  if (game.mode === "online") {
    scoreChange.textContent = "好友局";
    scoreDetail.textContent = "好友对战暂不计入排位积分";
  } else {
    scoreDetail.textContent = `黄金 III · ${playerStats.score}`;
  }

  if (game.mode === "ranked") {
    playerStats.games += 1;
    if (isWin) {
      playerStats.wins += 1;
      playerStats.score += 25;
    } else if (isDraw) {
      playerStats.score += 2;
    } else {
      playerStats.score = Math.max(0, playerStats.score - 12);
    }
    saveStats();
    updateProfileStats();
  }

  openBackdrop(resultBackdrop);
}

function startRankedMatch() {
  showToast("正在寻找实力相近的棋友…");
  window.setTimeout(() => startGame("ranked", "糯米团"), 700);
}

function setRankingPeriod(period) {
  const labels = {
    daily: ["今日排行已更新", "612", "594", "571"],
    weekly: ["本周排行已更新", "1,286", "1,174", "1,069"],
    total: ["总榜排行已更新", "8,920", "8,614", "8,106"],
  };
  const values = labels[period];
  document.querySelectorAll("[data-rank-tab]").forEach((button) => {
    button.classList.toggle("active", button.dataset.rankTab === period);
  });
  document.querySelectorAll(".podium-user small").forEach((score, index) => {
    score.textContent = values[index + 1];
  });
  showToast(values[0]);
}

document.querySelector("#createRoomButton").addEventListener("click", createOnlineRoom);
document.querySelector("#rankButton").addEventListener("click", startRankedMatch);
document.querySelector("#viewRankingButton").addEventListener("click", () => showView("ranking"));
document.querySelector("#joinRoomButton").addEventListener("click", () => openBackdrop(joinBackdrop));
document.querySelector("#sheetClose").addEventListener("click", () => closeBackdrop(sheet));
document.querySelector("#joinClose").addEventListener("click", () => closeBackdrop(joinBackdrop));
document.querySelector("#shareButton").addEventListener("click", shareRoom);
document.querySelector("#copyButton").addEventListener("click", () => copyText(roomLink, "邀请链接已复制"));
document.querySelector("#roomShareButton").addEventListener("click", () => openBackdrop(sheet));
document.querySelector("#roomCopyButton").addEventListener("click", () => copyText(online.roomCode, "房间号已复制"));
document.querySelector("#confirmJoinButton").addEventListener("click", async () => {
  const code = document.querySelector("#roomCodeInput").value.trim().replace(/\s/g, "").toUpperCase();
  if (!/^[A-HJ-NP-Z2-9]{6}$/.test(code)) {
    showToast("请输入正确的房间号");
    return;
  }
  showToast(`正在加入房间 ${code}`);
  await joinOnlineRoom(code);
});

document.querySelectorAll(".back-home").forEach((button) => button.addEventListener("click", () => showView("home")));
document.querySelector(".exit-game").addEventListener("click", () => {
  if (game.moves.length && !game.ended && !window.confirm("棋局还没有结束，确定退出吗？")) return;
  if (game.mode === "online") {
    if (!game.ended) sendRoomAction({ type: "resign" });
    online.manualClose = true;
    window.setTimeout(leaveOnlineRoom, 80);
    closeBackdrop(resultBackdrop);
  }
  showView("home");
});

navItems.forEach((item) => {
  item.addEventListener("click", () => {
    if (item.dataset.target === "game") startRankedMatch();
    else showView(item.dataset.target);
  });
});

document.querySelectorAll("[data-rank-tab]").forEach((button) => {
  button.addEventListener("click", () => setRankingPeriod(button.dataset.rankTab));
});

canvas.addEventListener("pointerup", handleBoardPointer);
document.querySelector("#undoButton").addEventListener("click", undoMove);
document.querySelector("#emojiButton").addEventListener("click", () => showToast("你向对手发送了：好棋！"));
document.querySelector("#resignButton").addEventListener("click", () => {
  if (!game.moves.length || game.ended) {
    showToast("对局还没有开始");
    return;
  }
  if (!window.confirm("确定认输这一局吗？")) return;
  if (game.mode === "online") sendRoomAction({ type: "resign" });
  else showResult(2, true);
});

document.querySelector("#soundButton").addEventListener("click", () => {
  soundEnabled = !soundEnabled;
  showToast(soundEnabled ? "已开启落子音效" : "已关闭落子音效");
});

document.querySelector("#playAgainButton").addEventListener("click", () => {
  if (game.mode === "online") {
    if (sendRoomAction({ type: "rematch" })) {
      closeBackdrop(resultBackdrop);
      showToast("已申请再来一局，等待对手确认");
    }
    return;
  }
  resetGame();
});
document.querySelector("#resultHomeButton").addEventListener("click", () => {
  closeBackdrop(resultBackdrop);
  if (game.mode === "online") leaveOnlineRoom();
  showView("home");
});

document.querySelector("#roomMoreButton").addEventListener("click", () => showToast("房间会在结束 30 分钟后自动关闭"));
document.querySelector("#settingsButton").addEventListener("click", () => showToast("音效、震动和隐私设置已预留"));
document.querySelector("#editNameButton").addEventListener("click", () => showToast("昵称编辑功能已预留"));

[sheet, joinBackdrop].forEach((backdrop) => {
  backdrop.addEventListener("click", (event) => {
    if (event.target === backdrop) closeBackdrop(backdrop);
  });
});

updateProfileStats();
drawBoard();

function restoreSharedRoom() {
  const params = new URLSearchParams(window.location.search);
  const code = (params.get("room") || "").trim().toUpperCase();
  if (!/^[A-HJ-NP-Z2-9]{6}$/.test(code)) return;

  document.querySelector("#roomCodeInput").value = code;
  const session = readRoomSession(code);
  if (!session?.token) {
    openBackdrop(joinBackdrop);
    return;
  }

  online.token = session.token;
  online.color = session.color;
  online.manualClose = false;
  updateRoomCode(code);
  resetLobby();
  showView("room");
  connectRoomSocket();
}

restoreSharedRoom();
