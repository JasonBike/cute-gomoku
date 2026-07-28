package game

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	boardSize      = 15
	roomCodeLength = 6
	roomAlphabet   = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

type coordinate struct {
	Row    int `json:"row"`
	Column int `json:"column"`
}

type move struct {
	coordinate
	Player int `json:"player"`
}

type player struct {
	Token     string
	UserID    string
	Name      string
	Color     int
	Connected bool
	Rematch   bool
	LastChat  time.Time
	conn      *websocket.Conn
}

type room struct {
	mu            sync.Mutex
	Code          string
	Board         [boardSize][boardSize]int
	Players       map[string]*player
	Turn          int
	Status        string
	Winner        int
	Moves         []move
	WinningLine   []coordinate
	UndoRequester int
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Server struct {
	webFS    fs.FS
	roomsMu  sync.RWMutex
	rooms    map[string]*room
	identity *identityStore
	upgrader websocket.Upgrader
}

type roomRequest struct {
	Name        string `json:"name"`
	PlayerToken string `json:"playerToken"`
}

type roomResponse struct {
	RoomCode    string `json:"roomCode"`
	PlayerToken string `json:"playerToken"`
	Color       int    `json:"color"`
}

type roomSummary struct {
	RoomCode       string `json:"roomCode"`
	HostName       string `json:"hostName"`
	Status         string `json:"status"`
	PlayerCount    int    `json:"playerCount"`
	ConnectedCount int    `json:"connectedCount"`
	MoveCount      int    `json:"moveCount"`
	Joinable       bool   `json:"joinable"`
	CreatedAt      int64  `json:"createdAt"`
}

type roomListResponse struct {
	Rooms []roomSummary `json:"rooms"`
}

type clientMessage struct {
	Type     string `json:"type"`
	Row      int    `json:"row"`
	Column   int    `json:"column"`
	Text     string `json:"text"`
	Accepted bool   `json:"accepted"`
}

type playerState struct {
	Name      string `json:"name"`
	Color     int    `json:"color"`
	Connected bool   `json:"connected"`
	Rematch   bool   `json:"rematch"`
}

type roomState struct {
	Type          string                    `json:"type"`
	RoomCode      string                    `json:"roomCode"`
	Status        string                    `json:"status"`
	Board         [boardSize][boardSize]int `json:"board"`
	Turn          int                       `json:"turn"`
	Winner        int                       `json:"winner"`
	Moves         []move                    `json:"moves"`
	Players       []playerState             `json:"players"`
	WinningLine   []coordinate              `json:"winningLine,omitempty"`
	UndoRequester int                       `json:"undoRequester,omitempty"`
}

type errorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type chatMessage struct {
	Type string `json:"type"`
	From int    `json:"from"`
	Name string `json:"name"`
	Text string `json:"text"`
}

var allowedChatMessages = map[string]struct{}{
	"嗨，来一局！":   {},
	"好棋！":      {},
	"差一点！":     {},
	"认真起来了":    {},
	"慢慢想，不着急":  {},
	"再来一局！":    {},
	"你打的可太好了。": {},
	"我等的花儿都谢了": {},
}

func NewServer(webFS fs.FS) http.Handler {
	handler, err := NewServerWithDataFile(webFS, "")
	if err != nil {
		panic(err)
	}
	return handler
}

func NewServerWithDataFile(webFS fs.FS, dataPath string) (http.Handler, error) {
	identity, err := newIdentityStore(dataPath)
	if err != nil {
		return nil, err
	}
	server := &Server{
		webFS:    webFS,
		rooms:    make(map[string]*room),
		identity: identity,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(request *http.Request) bool {
				origin := request.Header.Get("Origin")
				if origin == "" {
					return true
				}
				parsed, err := url.Parse(origin)
				return err == nil && parsed.Host == request.Host
			},
		},
	}

	go server.cleanupRooms()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", server.health)
	mux.HandleFunc("GET /api/session", server.session)
	mux.HandleFunc("PATCH /api/me", server.updateProfile)
	mux.HandleFunc("GET /api/rooms", server.listRooms)
	mux.HandleFunc("POST /api/rooms", server.createRoom)
	mux.HandleFunc("POST /api/rooms/{code}/join", server.joinRoom)
	mux.HandleFunc("GET /ws", server.serveWebSocket)
	mux.Handle("GET /assets/", immutableAssets(http.FileServer(http.FS(webFS))))
	mux.HandleFunc("GET /", server.serveIndex)
	return securityHeaders(mux), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; img-src 'self' data:")
		next.ServeHTTP(response, request)
	})
}

func immutableAssets(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		next.ServeHTTP(response, request)
	})
}

func (server *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) session(response http.ResponseWriter, request *http.Request) {
	session, err := server.identity.getOrCreateSession(response, request)
	if err != nil {
		log.Printf("create session failed: %v", err)
		writeAPIError(response, http.StatusInternalServerError, "session_unavailable", "暂时无法创建棋手身份")
		return
	}
	writeJSON(response, http.StatusOK, session)
}

func (server *Server) updateProfile(response http.ResponseWriter, request *http.Request) {
	var body updateProfileRequest
	if err := decodeJSON(request, &body); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求内容不正确")
		return
	}
	body.Nickname = strings.TrimSpace(body.Nickname)
	nicknameLength := len([]rune(body.Nickname))
	if nicknameLength < 1 || nicknameLength > 12 {
		writeAPIError(response, http.StatusBadRequest, "invalid_nickname", "昵称需要在 1 到 12 个字之间")
		return
	}

	session, err := server.identity.getOrCreateSession(response, request)
	if err != nil {
		log.Printf("resolve profile session failed: %v", err)
		writeAPIError(response, http.StatusInternalServerError, "session_unavailable", "暂时无法识别棋手身份")
		return
	}
	user, err := server.identity.updateNickname(session.User.ID, body.Nickname)
	if err != nil {
		log.Printf("save profile failed: %v", err)
		writeAPIError(response, http.StatusInternalServerError, "profile_save_failed", "昵称保存失败，请稍后重试")
		return
	}
	server.updateRoomPlayerNames(user.ID, user.Nickname)
	writeJSON(response, http.StatusOK, sessionResponse{User: user, ExpiresAt: session.ExpiresAt})
}

func (server *Server) serveIndex(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	server.serveFile(response, request, "index.html", "text/html; charset=utf-8")
}

func (server *Server) serveFile(response http.ResponseWriter, request *http.Request, name, contentType string) {
	content, err := fs.ReadFile(server.webFS, name)
	if err != nil {
		http.Error(response, "页面资源不存在", http.StatusNotFound)
		return
	}
	response.Header().Set("Content-Type", contentType)
	response.Header().Set("Cache-Control", "no-cache")
	_, _ = response.Write(content)
}

func (server *Server) createRoom(response http.ResponseWriter, request *http.Request) {
	var body roomRequest
	if err := decodeJSON(request, &body); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求内容不正确")
		return
	}
	session, err := server.identity.getOrCreateSession(response, request)
	if err != nil {
		log.Printf("resolve room creator failed: %v", err)
		writeAPIError(response, http.StatusInternalServerError, "session_unavailable", "暂时无法识别棋手身份")
		return
	}

	code, err := server.newRoomCode()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, "code_generation_failed", "暂时无法创建房间")
		return
	}
	token, err := newToken()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, "token_generation_failed", "暂时无法创建房间")
		return
	}

	now := time.Now()
	gameRoom := &room{
		Code:      code,
		Players:   make(map[string]*player),
		Turn:      1,
		Status:    "waiting",
		CreatedAt: now,
		UpdatedAt: now,
	}
	gameRoom.Players[token] = &player{
		Token:  token,
		UserID: session.User.ID,
		Name:   session.User.Nickname,
		Color:  1,
	}

	server.roomsMu.Lock()
	server.rooms[code] = gameRoom
	server.roomsMu.Unlock()

	writeJSON(response, http.StatusCreated, roomResponse{RoomCode: code, PlayerToken: token, Color: 1})
}

func (server *Server) listRooms(response http.ResponseWriter, _ *http.Request) {
	result := roomListResponse{Rooms: make([]roomSummary, 0)}

	server.roomsMu.RLock()
	for _, gameRoom := range server.rooms {
		gameRoom.mu.Lock()
		hostName := "神秘棋手"
		connectedCount := 0
		for _, roomPlayer := range gameRoom.Players {
			if roomPlayer.Color == 1 {
				hostName = roomPlayer.Name
			}
			if roomPlayer.Connected {
				connectedCount++
			}
		}
		if connectedCount > 0 {
			result.Rooms = append(result.Rooms, roomSummary{
				RoomCode:       gameRoom.Code,
				HostName:       hostName,
				Status:         gameRoom.Status,
				PlayerCount:    connectedCount,
				ConnectedCount: connectedCount,
				MoveCount:      len(gameRoom.Moves),
				Joinable:       roomJoinable(gameRoom),
				CreatedAt:      gameRoom.CreatedAt.UnixMilli(),
			})
		}
		gameRoom.mu.Unlock()
	}
	server.roomsMu.RUnlock()

	sort.Slice(result.Rooms, func(left, right int) bool {
		if result.Rooms[left].Joinable != result.Rooms[right].Joinable {
			return result.Rooms[left].Joinable
		}
		return result.Rooms[left].CreatedAt > result.Rooms[right].CreatedAt
	})
	writeJSON(response, http.StatusOK, result)
}

func (server *Server) joinRoom(response http.ResponseWriter, request *http.Request) {
	code := normalizeRoomCode(request.PathValue("code"))
	gameRoom := server.getRoom(code)
	if gameRoom == nil {
		writeAPIError(response, http.StatusNotFound, "room_not_found", "房间不存在或已经过期")
		return
	}

	var body roomRequest
	if err := decodeJSON(request, &body); err != nil {
		writeAPIError(response, http.StatusBadRequest, "invalid_request", "请求内容不正确")
		return
	}
	session, err := server.identity.getOrCreateSession(response, request)
	if err != nil {
		log.Printf("resolve joining player failed: %v", err)
		writeAPIError(response, http.StatusInternalServerError, "session_unavailable", "暂时无法识别棋手身份")
		return
	}

	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()
	if existingPlayer := gameRoom.Players[body.PlayerToken]; body.PlayerToken != "" && existingPlayer != nil {
		gameRoom.UpdatedAt = time.Now()
		writeJSON(response, http.StatusOK, roomResponse{
			RoomCode:    code,
			PlayerToken: existingPlayer.Token,
			Color:       existingPlayer.Color,
		})
		return
	}

	joiningFinishedRoom := gameRoom.Status == "finished" && connectedPlayerCount(gameRoom) == 1
	if gameRoom.Status != "waiting" && !joiningFinishedRoom {
		writeAPIError(response, http.StatusConflict, "game_started", "棋局已经开始")
		return
	}
	if gameRoom.Status == "waiting" && len(gameRoom.Players) >= 2 {
		writeAPIError(response, http.StatusConflict, "room_full", "这个房间已经坐满了")
		return
	}

	token, err := newToken()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, "token_generation_failed", "暂时无法加入房间")
		return
	}
	color := 2
	if joiningFinishedRoom {
		for oldToken, roomPlayer := range gameRoom.Players {
			if roomPlayer.Connected {
				color = oppositeColor(roomPlayer.Color)
				continue
			}
			color = roomPlayer.Color
			delete(gameRoom.Players, oldToken)
		}
		resetRoomToWaitingLocked(gameRoom)
	}
	gameRoom.Players[token] = &player{
		Token:  token,
		UserID: session.User.ID,
		Name:   session.User.Nickname,
		Color:  color,
	}
	gameRoom.UpdatedAt = time.Now()
	if joiningFinishedRoom {
		server.broadcastLocked(gameRoom)
	}
	writeJSON(response, http.StatusOK, roomResponse{RoomCode: code, PlayerToken: token, Color: color})
}

func (server *Server) serveWebSocket(response http.ResponseWriter, request *http.Request) {
	code := normalizeRoomCode(request.URL.Query().Get("room"))
	token := request.URL.Query().Get("token")
	gameRoom := server.getRoom(code)
	if gameRoom == nil {
		writeAPIError(response, http.StatusNotFound, "room_not_found", "房间不存在或已经过期")
		return
	}

	gameRoom.mu.Lock()
	currentPlayer := gameRoom.Players[token]
	gameRoom.mu.Unlock()
	if currentPlayer == nil {
		writeAPIError(response, http.StatusUnauthorized, "invalid_player", "玩家身份无效")
		return
	}

	connection, err := server.upgrader.Upgrade(response, request, nil)
	if err != nil {
		log.Printf("websocket upgrade failed: %v", err)
		return
	}
	connection.SetReadLimit(4096)
	_ = connection.SetReadDeadline(time.Now().Add(70 * time.Second))
	connection.SetPongHandler(func(string) error {
		return connection.SetReadDeadline(time.Now().Add(70 * time.Second))
	})

	gameRoom.mu.Lock()
	if currentPlayer.conn != nil {
		_ = currentPlayer.conn.Close()
	}
	currentPlayer.conn = connection
	currentPlayer.Connected = true
	gameRoom.UpdatedAt = time.Now()
	if gameRoom.Status == "waiting" && len(gameRoom.Players) == 2 && bothPlayersConnected(gameRoom) {
		gameRoom.Status = "playing"
	}
	server.broadcastLocked(gameRoom)
	gameRoom.mu.Unlock()

	go server.pingLoop(gameRoom, currentPlayer, connection)
	server.readLoop(gameRoom, currentPlayer, connection)
}

func (server *Server) readLoop(gameRoom *room, currentPlayer *player, connection *websocket.Conn) {
	defer func() {
		gameRoom.mu.Lock()
		if currentPlayer.conn == connection {
			currentPlayer.conn = nil
			currentPlayer.Connected = false
			gameRoom.UndoRequester = 0
			clearRematchRequests(gameRoom)
			gameRoom.UpdatedAt = time.Now()
			server.broadcastLocked(gameRoom)
		}
		gameRoom.mu.Unlock()
		_ = connection.Close()
	}()

	for {
		var message clientMessage
		if err := connection.ReadJSON(&message); err != nil {
			return
		}
		server.handleClientMessage(gameRoom, currentPlayer, connection, message)
	}
}

func (server *Server) pingLoop(gameRoom *room, currentPlayer *player, connection *websocket.Conn) {
	ticker := time.NewTicker(25 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		gameRoom.mu.Lock()
		if currentPlayer.conn != connection {
			gameRoom.mu.Unlock()
			return
		}
		_ = connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
		err := connection.WriteMessage(websocket.PingMessage, nil)
		gameRoom.mu.Unlock()
		if err != nil {
			_ = connection.Close()
			return
		}
	}
}

func (server *Server) handleClientMessage(gameRoom *room, currentPlayer *player, connection *websocket.Conn, message clientMessage) {
	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()

	switch message.Type {
	case "move":
		if code, text := validateMove(gameRoom, currentPlayer, message.Row, message.Column); code != "" {
			writeSocketError(connection, code, text)
			return
		}
		gameRoom.Board[message.Row][message.Column] = currentPlayer.Color
		gameRoom.Moves = append(gameRoom.Moves, move{
			coordinate: coordinate{Row: message.Row, Column: message.Column},
			Player:     currentPlayer.Color,
		})
		gameRoom.WinningLine = findWinningLine(gameRoom.Board, message.Row, message.Column, currentPlayer.Color)
		if len(gameRoom.WinningLine) == 2 {
			gameRoom.Winner = currentPlayer.Color
			gameRoom.Status = "finished"
		} else if len(gameRoom.Moves) == boardSize*boardSize {
			gameRoom.Winner = 0
			gameRoom.Status = "finished"
		} else {
			gameRoom.Turn = oppositeColor(gameRoom.Turn)
		}
	case "resign":
		if gameRoom.Status != "playing" {
			writeSocketError(connection, "game_not_playing", "当前没有进行中的棋局")
			return
		}
		gameRoom.Winner = oppositeColor(currentPlayer.Color)
		gameRoom.Status = "finished"
		gameRoom.UndoRequester = 0
	case "rematch":
		if gameRoom.Status != "finished" {
			writeSocketError(connection, "game_not_finished", "棋局结束后才能再来一局")
			return
		}
		if !bothPlayersConnected(gameRoom) {
			writeSocketError(connection, "opponent_offline", "对手暂时离线，无法申请再来一局")
			return
		}
		currentPlayer.Rematch = true
		if bothPlayersWantRematch(gameRoom) {
			resetRoomLocked(gameRoom)
		}
	case "undo_request":
		if gameRoom.Status != "playing" {
			writeSocketError(connection, "game_not_playing", "棋局进行中才能申请悔棋")
			return
		}
		if !bothPlayersConnected(gameRoom) {
			writeSocketError(connection, "opponent_offline", "对手暂时离线，无法申请悔棋")
			return
		}
		if gameRoom.UndoRequester != 0 {
			writeSocketError(connection, "undo_pending", "已经有一条悔棋申请等待处理")
			return
		}
		if !hasMoveByPlayer(gameRoom, currentPlayer.Color) {
			writeSocketError(connection, "nothing_to_undo", "你还没有可以撤回的棋子")
			return
		}
		gameRoom.UndoRequester = currentPlayer.Color
	case "undo_response":
		if gameRoom.UndoRequester == 0 {
			writeSocketError(connection, "no_undo_request", "当前没有待处理的悔棋申请")
			return
		}
		if gameRoom.UndoRequester == currentPlayer.Color {
			writeSocketError(connection, "cannot_review_own_undo", "不能处理自己的悔棋申请")
			return
		}
		requester := gameRoom.UndoRequester
		if message.Accepted {
			rollbackToPlayerLocked(gameRoom, requester)
		} else {
			gameRoom.UndoRequester = 0
		}
	case "chat":
		text := strings.TrimSpace(message.Text)
		if _, allowed := allowedChatMessages[text]; !allowed {
			writeSocketError(connection, "invalid_chat", "不支持这条快捷消息")
			return
		}
		if !bothPlayersConnected(gameRoom) {
			writeSocketError(connection, "opponent_offline", "对手暂时离线，消息没有发出")
			return
		}
		if time.Since(currentPlayer.LastChat) < 2*time.Second {
			writeSocketError(connection, "chat_too_fast", "说得太快啦，稍等一下")
			return
		}
		currentPlayer.LastChat = time.Now()
		server.sendChatLocked(gameRoom, currentPlayer, text)
		return
	default:
		writeSocketError(connection, "unknown_action", "不支持的操作")
		return
	}

	gameRoom.UpdatedAt = time.Now()
	server.broadcastLocked(gameRoom)
}

func (server *Server) sendChatLocked(gameRoom *room, sender *player, text string) {
	message := chatMessage{
		Type: "chat",
		From: sender.Color,
		Name: sender.Name,
		Text: text,
	}
	for _, roomPlayer := range gameRoom.Players {
		if roomPlayer.conn == nil {
			continue
		}
		_ = roomPlayer.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := roomPlayer.conn.WriteJSON(message); err != nil {
			_ = roomPlayer.conn.Close()
		}
	}
}

func validateMove(gameRoom *room, currentPlayer *player, row, column int) (string, string) {
	if gameRoom.Status != "playing" {
		return "game_not_playing", "棋局还没有开始"
	}
	if !bothPlayersConnected(gameRoom) {
		return "opponent_offline", "对手暂时离线，请等待重连"
	}
	if gameRoom.UndoRequester != 0 {
		return "undo_pending", "请先处理当前的悔棋申请"
	}
	if gameRoom.Turn != currentPlayer.Color {
		return "not_your_turn", "还没有轮到你"
	}
	if row < 0 || row >= boardSize || column < 0 || column >= boardSize {
		return "invalid_position", "落子位置超出棋盘"
	}
	if gameRoom.Board[row][column] != 0 {
		return "position_occupied", "这里已经有棋子了"
	}
	return "", ""
}

func resetRoomLocked(gameRoom *room) {
	resetRoomToWaitingLocked(gameRoom)
	gameRoom.Status = "playing"
}

func resetRoomToWaitingLocked(gameRoom *room) {
	gameRoom.Board = [boardSize][boardSize]int{}
	gameRoom.Moves = make([]move, 0)
	gameRoom.WinningLine = nil
	gameRoom.Turn = 1
	gameRoom.Winner = 0
	gameRoom.UndoRequester = 0
	gameRoom.Status = "waiting"
	for _, roomPlayer := range gameRoom.Players {
		roomPlayer.Rematch = false
	}
}

func clearRematchRequests(gameRoom *room) {
	for _, roomPlayer := range gameRoom.Players {
		roomPlayer.Rematch = false
	}
}

func hasMoveByPlayer(gameRoom *room, color int) bool {
	for index := len(gameRoom.Moves) - 1; index >= 0; index-- {
		if gameRoom.Moves[index].Player == color {
			return true
		}
	}
	return false
}

func rollbackToPlayerLocked(gameRoom *room, color int) {
	start := -1
	for index := len(gameRoom.Moves) - 1; index >= 0; index-- {
		if gameRoom.Moves[index].Player == color {
			start = index
			break
		}
	}
	if start < 0 {
		gameRoom.UndoRequester = 0
		return
	}
	for _, removed := range gameRoom.Moves[start:] {
		gameRoom.Board[removed.Row][removed.Column] = 0
	}
	gameRoom.Moves = append([]move{}, gameRoom.Moves[:start]...)
	gameRoom.WinningLine = nil
	gameRoom.Winner = 0
	gameRoom.Turn = color
	gameRoom.Status = "playing"
	gameRoom.UndoRequester = 0
}

func (server *Server) broadcastLocked(gameRoom *room) {
	state := roomState{
		Type:          "state",
		RoomCode:      gameRoom.Code,
		Status:        gameRoom.Status,
		Board:         gameRoom.Board,
		Turn:          gameRoom.Turn,
		Winner:        gameRoom.Winner,
		Moves:         append([]move{}, gameRoom.Moves...),
		WinningLine:   append([]coordinate(nil), gameRoom.WinningLine...),
		UndoRequester: gameRoom.UndoRequester,
	}
	for _, roomPlayer := range gameRoom.Players {
		state.Players = append(state.Players, playerState{
			Name:      roomPlayer.Name,
			Color:     roomPlayer.Color,
			Connected: roomPlayer.Connected,
			Rematch:   roomPlayer.Rematch,
		})
	}

	for _, roomPlayer := range gameRoom.Players {
		if roomPlayer.conn == nil {
			continue
		}
		_ = roomPlayer.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := roomPlayer.conn.WriteJSON(state); err != nil {
			_ = roomPlayer.conn.Close()
		}
	}
}

func writeSocketError(connection *websocket.Conn, code, message string) {
	_ = connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = connection.WriteJSON(errorMessage{Type: "error", Code: code, Message: message})
}

func (server *Server) getRoom(code string) *room {
	server.roomsMu.RLock()
	defer server.roomsMu.RUnlock()
	return server.rooms[code]
}

func (server *Server) updateRoomPlayerNames(userID, nickname string) {
	server.roomsMu.RLock()
	defer server.roomsMu.RUnlock()
	for _, gameRoom := range server.rooms {
		gameRoom.mu.Lock()
		changed := false
		for _, roomPlayer := range gameRoom.Players {
			if roomPlayer.UserID == userID && roomPlayer.Name != nickname {
				roomPlayer.Name = nickname
				changed = true
			}
		}
		if changed {
			gameRoom.UpdatedAt = time.Now()
			server.broadcastLocked(gameRoom)
		}
		gameRoom.mu.Unlock()
	}
}

func (server *Server) newRoomCode() (string, error) {
	for attempt := 0; attempt < 20; attempt++ {
		code, err := randomString(roomCodeLength, roomAlphabet)
		if err != nil {
			return "", err
		}
		if server.getRoom(code) == nil {
			return code, nil
		}
	}
	return "", errors.New("unable to allocate unique room code")
}

func newToken() (string, error) {
	bytes := make([]byte, 24)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func randomString(length int, alphabet string) (string, error) {
	if length < 1 || len(alphabet) < 2 || len(alphabet) > 256 {
		return "", errors.New("invalid random string parameters")
	}
	result := make([]byte, length)
	buffer := make([]byte, length*2)
	maxValid := 256 - (256 % len(alphabet))
	position := 0
	for position < length {
		if _, err := rand.Read(buffer); err != nil {
			return "", err
		}
		for _, value := range buffer {
			if int(value) >= maxValid {
				continue
			}
			result[position] = alphabet[int(value)%len(alphabet)]
			position++
			if position == length {
				break
			}
		}
	}
	return string(result), nil
}

func findWinningLine(board [boardSize][boardSize]int, row, column, color int) []coordinate {
	directions := [][2]int{{1, 0}, {0, 1}, {1, 1}, {1, -1}}
	for _, direction := range directions {
		stones := []coordinate{{Row: row, Column: column}}
		for _, sign := range []int{-1, 1} {
			nextRow := row + direction[0]*sign
			nextColumn := column + direction[1]*sign
			for inBoard(nextRow, nextColumn) && board[nextRow][nextColumn] == color {
				point := coordinate{Row: nextRow, Column: nextColumn}
				if sign == -1 {
					stones = append([]coordinate{point}, stones...)
				} else {
					stones = append(stones, point)
				}
				nextRow += direction[0] * sign
				nextColumn += direction[1] * sign
			}
		}
		if len(stones) >= 5 {
			return []coordinate{stones[0], stones[len(stones)-1]}
		}
	}
	return nil
}

func inBoard(row, column int) bool {
	return row >= 0 && row < boardSize && column >= 0 && column < boardSize
}

func oppositeColor(color int) int {
	if color == 1 {
		return 2
	}
	return 1
}

func bothPlayersConnected(gameRoom *room) bool {
	if len(gameRoom.Players) != 2 {
		return false
	}
	for _, roomPlayer := range gameRoom.Players {
		if !roomPlayer.Connected {
			return false
		}
	}
	return true
}

func connectedPlayerCount(gameRoom *room) int {
	count := 0
	for _, roomPlayer := range gameRoom.Players {
		if roomPlayer.Connected {
			count++
		}
	}
	return count
}

func roomJoinable(gameRoom *room) bool {
	if gameRoom.Status == "waiting" {
		return len(gameRoom.Players) < 2
	}
	return gameRoom.Status == "finished" && connectedPlayerCount(gameRoom) == 1
}

func bothPlayersWantRematch(gameRoom *room) bool {
	if len(gameRoom.Players) != 2 {
		return false
	}
	for _, roomPlayer := range gameRoom.Players {
		if !roomPlayer.Rematch {
			return false
		}
	}
	return true
}

func normalizeRoomCode(value string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	return strings.ReplaceAll(value, " ", "")
}

func normalizeName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "神秘棋手"
	}
	runes := []rune(value)
	if len(runes) > 12 {
		value = string(runes[:12])
	}
	return value
}

func decodeJSON(request *http.Request, target any) error {
	defer request.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(request.Body, 4096))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(response http.ResponseWriter, status int, value any) {
	response.Header().Set("Content-Type", "application/json; charset=utf-8")
	response.Header().Set("Cache-Control", "no-store")
	response.WriteHeader(status)
	if err := json.NewEncoder(response).Encode(value); err != nil {
		log.Printf("encode response failed: %v", err)
	}
}

func writeAPIError(response http.ResponseWriter, status int, code, message string) {
	writeJSON(response, status, errorMessage{Type: "error", Code: code, Message: message})
}

func (server *Server) cleanupRooms() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for now := range ticker.C {
		server.roomsMu.Lock()
		for code, gameRoom := range server.rooms {
			gameRoom.mu.Lock()
			expired := roomExpired(gameRoom, now)
			gameRoom.mu.Unlock()
			if expired {
				delete(server.rooms, code)
			}
		}
		server.roomsMu.Unlock()
	}
}

func roomExpired(gameRoom *room, now time.Time) bool {
	for _, roomPlayer := range gameRoom.Players {
		if roomPlayer.Connected {
			return false
		}
	}
	return now.Sub(gameRoom.UpdatedAt) > 30*time.Minute
}

func (state roomState) String() string {
	return fmt.Sprintf("room=%s status=%s turn=%d moves=%d", state.RoomCode, state.Status, state.Turn, len(state.Moves))
}
