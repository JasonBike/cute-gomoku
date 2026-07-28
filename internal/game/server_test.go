package game

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestFindWinningLine(t *testing.T) {
	tests := []struct {
		name   string
		points []coordinate
		last   coordinate
	}{
		{
			name:   "horizontal",
			points: []coordinate{{7, 3}, {7, 4}, {7, 5}, {7, 6}, {7, 7}},
			last:   coordinate{7, 5},
		},
		{
			name:   "vertical",
			points: []coordinate{{3, 6}, {4, 6}, {5, 6}, {6, 6}, {7, 6}},
			last:   coordinate{6, 6},
		},
		{
			name:   "downward diagonal",
			points: []coordinate{{2, 2}, {3, 3}, {4, 4}, {5, 5}, {6, 6}},
			last:   coordinate{4, 4},
		},
		{
			name:   "upward diagonal",
			points: []coordinate{{2, 8}, {3, 7}, {4, 6}, {5, 5}, {6, 4}},
			last:   coordinate{3, 7},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var board [boardSize][boardSize]int
			for _, point := range test.points {
				board[point.Row][point.Column] = 1
			}
			line := findWinningLine(board, test.last.Row, test.last.Column, 1)
			if len(line) != 2 {
				t.Fatalf("expected winning line, got %#v", line)
			}
		})
	}
}

func TestFindWinningLineRequiresFive(t *testing.T) {
	var board [boardSize][boardSize]int
	for column := 3; column < 7; column++ {
		board[8][column] = 2
	}
	if line := findWinningLine(board, 8, 5, 2); line != nil {
		t.Fatalf("expected no winning line, got %#v", line)
	}
}

func TestCreateAndJoinRoom(t *testing.T) {
	handler := NewServer(os.DirFS("../.."))

	createRequest := httptest.NewRequest(http.MethodPost, "/api/rooms", bytes.NewBufferString(`{"name":"小桃子"}`))
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create room returned %d: %s", createResponse.Code, createResponse.Body.String())
	}

	var created roomResponse
	if err := json.NewDecoder(createResponse.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	if len(created.RoomCode) != roomCodeLength {
		t.Fatalf("unexpected room code %q", created.RoomCode)
	}
	if created.PlayerToken == "" || created.Color != 1 {
		t.Fatalf("unexpected creator response: %#v", created)
	}

	joinRequest := httptest.NewRequest(
		http.MethodPost,
		"/api/rooms/"+created.RoomCode+"/join",
		bytes.NewBufferString(`{"name":"青团兔"}`),
	)
	joinRequest.SetPathValue("code", created.RoomCode)
	joinResponse := httptest.NewRecorder()
	handler.ServeHTTP(joinResponse, joinRequest)
	if joinResponse.Code != http.StatusOK {
		t.Fatalf("join room returned %d: %s", joinResponse.Code, joinResponse.Body.String())
	}

	var joined roomResponse
	if err := json.NewDecoder(joinResponse.Body).Decode(&joined); err != nil {
		t.Fatal(err)
	}
	if joined.Color != 2 || joined.PlayerToken == created.PlayerToken {
		t.Fatalf("unexpected join response: %#v", joined)
	}
}

func TestRoomRejectsThirdPlayer(t *testing.T) {
	handler := NewServer(os.DirFS("../.."))
	createResponse := httptest.NewRecorder()
	handler.ServeHTTP(
		createResponse,
		httptest.NewRequest(http.MethodPost, "/api/rooms", strings.NewReader(`{"name":"A"}`)),
	)
	var created roomResponse
	_ = json.NewDecoder(createResponse.Body).Decode(&created)

	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(
			http.MethodPost,
			"/api/rooms/"+created.RoomCode+"/join",
			strings.NewReader(`{"name":"B"}`),
		)
		request.SetPathValue("code", created.RoomCode)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if attempt == 0 && response.Code != http.StatusOK {
			t.Fatalf("first join returned %d", response.Code)
		}
		if attempt == 1 && response.Code != http.StatusConflict {
			t.Fatalf("third player returned %d, want %d", response.Code, http.StatusConflict)
		}
	}
}

func TestWebSocketGameFlow(t *testing.T) {
	testServer := httptest.NewServer(NewServer(os.DirFS("../..")))
	defer testServer.Close()

	creator := requestRoom(t, testServer.URL, http.MethodPost, "/api/rooms", `{"name":"黑棋"}`)
	guest := requestRoom(
		t,
		testServer.URL,
		http.MethodPost,
		"/api/rooms/"+creator.RoomCode+"/join",
		`{"name":"白棋"}`,
	)

	black := dialRoom(t, testServer.URL, creator.RoomCode, creator.PlayerToken)
	defer black.Close()
	firstState := readState(t, black)
	if firstState.Status != "waiting" {
		t.Fatalf("first player status = %q, want waiting", firstState.Status)
	}

	white := dialRoom(t, testServer.URL, creator.RoomCode, guest.PlayerToken)
	defer white.Close()
	if state := readState(t, black); state.Status != "playing" {
		t.Fatalf("black status = %q, want playing", state.Status)
	}
	if state := readState(t, white); state.Status != "playing" {
		t.Fatalf("white status = %q, want playing", state.Status)
	}

	blackColumns := []int{7, 8, 9, 10, 11}
	whiteColumns := []int{0, 1, 2, 3}
	for turn, column := range blackColumns {
		writeMove(t, black, 7, column)
		blackState := readState(t, black)
		_ = readState(t, white)
		if turn == len(blackColumns)-1 {
			if blackState.Status != "finished" || blackState.Winner != 1 || len(blackState.WinningLine) != 2 {
				t.Fatalf("unexpected final state: %#v", blackState)
			}
			break
		}

		writeMove(t, white, 0, whiteColumns[turn])
		_ = readState(t, black)
		_ = readState(t, white)
	}
}

func requestRoom(t *testing.T, baseURL, method, path, body string) roomResponse {
	t.Helper()
	request, err := http.NewRequest(method, baseURL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		t.Fatalf("%s returned %d", path, response.StatusCode)
	}
	var result roomResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func dialRoom(t *testing.T, baseURL, code, token string) *websocket.Conn {
	t.Helper()
	socketURL := "ws" + strings.TrimPrefix(baseURL, "http") + "/ws?room=" + code + "&token=" + token
	connection, response, err := websocket.DefaultDialer.Dial(socketURL, nil)
	if err != nil {
		if response != nil {
			t.Fatalf("dial returned %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	return connection
}

func readState(t *testing.T, connection *websocket.Conn) roomState {
	t.Helper()
	_ = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	var state roomState
	if err := connection.ReadJSON(&state); err != nil {
		t.Fatal(err)
	}
	if state.Type != "state" {
		t.Fatalf("unexpected message: %#v", state)
	}
	return state
}

func writeMove(t *testing.T, connection *websocket.Conn, row, column int) {
	t.Helper()
	_ = connection.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := connection.WriteJSON(clientMessage{Type: "move", Row: row, Column: column}); err != nil {
		t.Fatal(err)
	}
}
