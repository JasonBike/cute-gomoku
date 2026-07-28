package game

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	Name      string
	Color     int
	Connected bool
	Rematch   bool
	conn      *websocket.Conn
}

type room struct {
	mu          sync.Mutex
	Code        string
	Board       [boardSize][boardSize]int
	Players     map[string]*player
	Turn        int
	Status      string
	Winner      int
	Moves       []move
	WinningLine []coordinate
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Server struct {
	webDir   string
	roomsMu  sync.RWMutex
	rooms    map[string]*room
	upgrader websocket.Upgrader
}

type roomRequest struct {
	Name string `json:"name"`
}

type roomResponse struct {
	RoomCode    string `json:"roomCode"`
	PlayerToken string `json:"playerToken"`
	Color       int    `json:"color"`
}

type clientMessage struct {
	Type   string `json:"type"`
	Row    int    `json:"row"`
	Column int    `json:"column"`
}

type playerState struct {
	Name      string `json:"name"`
	Color     int    `json:"color"`
	Connected bool   `json:"connected"`
	Rematch   bool   `json:"rematch"`
}

type roomState struct {
	Type        string                    `json:"type"`
	RoomCode    string                    `json:"roomCode"`
	Status      string                    `json:"status"`
	Board       [boardSize][boardSize]int `json:"board"`
	Turn        int                       `json:"turn"`
	Winner      int                       `json:"winner"`
	Moves       []move                    `json:"moves"`
	Players     []playerState             `json:"players"`
	WinningLine []coordinate              `json:"winningLine,omitempty"`
}

type errorMessage struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

func NewServer(webDir string) http.Handler {
	server := &Server{
		webDir: webDir,
		rooms:  make(map[string]*room),
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
	mux.HandleFunc("POST /api/rooms", server.createRoom)
	mux.HandleFunc("POST /api/rooms/{code}/join", server.joinRoom)
	mux.HandleFunc("GET /ws", server.serveWebSocket)
	mux.HandleFunc("GET /styles.css", server.serveAsset("styles.css", "text/css; charset=utf-8"))
	mux.HandleFunc("GET /app.js", server.serveAsset("app.js", "application/javascript; charset=utf-8"))
	mux.HandleFunc("GET /", server.serveIndex)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		response.Header().Set("X-Content-Type-Options", "nosniff")
		response.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		response.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; connect-src 'self' ws: wss:; img-src 'self' data:")
		next.ServeHTTP(response, request)
	})
}

func (server *Server) health(response http.ResponseWriter, _ *http.Request) {
	writeJSON(response, http.StatusOK, map[string]string{"status": "ok"})
}

func (server *Server) serveIndex(response http.ResponseWriter, request *http.Request) {
	if request.URL.Path != "/" {
		http.NotFound(response, request)
		return
	}
	server.serveFile(response, request, "index.html", "text/html; charset=utf-8")
}

func (server *Server) serveAsset(name, contentType string) http.HandlerFunc {
	return func(response http.ResponseWriter, request *http.Request) {
		server.serveFile(response, request, name, contentType)
	}
}

func (server *Server) serveFile(response http.ResponseWriter, request *http.Request, name, contentType string) {
	path := filepath.Join(server.webDir, name)
	content, err := os.ReadFile(path)
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
	body.Name = normalizeName(body.Name)

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
	gameRoom.Players[token] = &player{Token: token, Name: body.Name, Color: 1}

	server.roomsMu.Lock()
	server.rooms[code] = gameRoom
	server.roomsMu.Unlock()

	writeJSON(response, http.StatusCreated, roomResponse{RoomCode: code, PlayerToken: token, Color: 1})
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
	body.Name = normalizeName(body.Name)
	token, err := newToken()
	if err != nil {
		writeAPIError(response, http.StatusInternalServerError, "token_generation_failed", "暂时无法加入房间")
		return
	}

	gameRoom.mu.Lock()
	defer gameRoom.mu.Unlock()
	if len(gameRoom.Players) >= 2 {
		writeAPIError(response, http.StatusConflict, "room_full", "这个房间已经坐满了")
		return
	}
	if gameRoom.Status != "waiting" {
		writeAPIError(response, http.StatusConflict, "game_started", "棋局已经开始")
		return
	}

	gameRoom.Players[token] = &player{Token: token, Name: body.Name, Color: 2}
	gameRoom.UpdatedAt = time.Now()
	writeJSON(response, http.StatusOK, roomResponse{RoomCode: code, PlayerToken: token, Color: 2})
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
	case "rematch":
		if gameRoom.Status != "finished" {
			writeSocketError(connection, "game_not_finished", "棋局结束后才能再来一局")
			return
		}
		currentPlayer.Rematch = true
		if bothPlayersWantRematch(gameRoom) {
			resetRoomLocked(gameRoom)
		}
	default:
		writeSocketError(connection, "unknown_action", "不支持的操作")
		return
	}

	gameRoom.UpdatedAt = time.Now()
	server.broadcastLocked(gameRoom)
}

func validateMove(gameRoom *room, currentPlayer *player, row, column int) (string, string) {
	if gameRoom.Status != "playing" {
		return "game_not_playing", "棋局还没有开始"
	}
	if !bothPlayersConnected(gameRoom) {
		return "opponent_offline", "对手暂时离线，请等待重连"
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
	gameRoom.Board = [boardSize][boardSize]int{}
	gameRoom.Moves = nil
	gameRoom.WinningLine = nil
	gameRoom.Turn = 1
	gameRoom.Winner = 0
	gameRoom.Status = "playing"
	for _, roomPlayer := range gameRoom.Players {
		roomPlayer.Rematch = false
	}
}

func (server *Server) broadcastLocked(gameRoom *room) {
	state := roomState{
		Type:        "state",
		RoomCode:    gameRoom.Code,
		Status:      gameRoom.Status,
		Board:       gameRoom.Board,
		Turn:        gameRoom.Turn,
		Winner:      gameRoom.Winner,
		Moves:       append([]move(nil), gameRoom.Moves...),
		WinningLine: append([]coordinate(nil), gameRoom.WinningLine...),
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
			age := now.Sub(gameRoom.UpdatedAt)
			expired := age > 2*time.Hour || (gameRoom.Status == "finished" && age > 30*time.Minute)
			gameRoom.mu.Unlock()
			if expired {
				delete(server.rooms, code)
			}
		}
		server.roomsMu.Unlock()
	}
}

func (state roomState) String() string {
	return fmt.Sprintf("room=%s status=%s turn=%d moves=%d", state.RoomCode, state.Status, state.Turn, len(state.Moves))
}
