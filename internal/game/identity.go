package game

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	identityStateVersion = 1
	sessionCookieName    = "qiyu_session"
	deviceCookieName     = "qiyu_buvid"
	sessionLifetime      = 24 * time.Hour
	deviceLifetime       = 30 * 24 * time.Hour
)

var defaultNicknames = []string{
	"桃桃小棋手",
	"青团兔兔",
	"糯米团子",
	"云朵小熊",
	"布丁喵喵",
	"柚子汽水",
	"奶糖团团",
	"栗子猫猫",
	"月亮兔兔",
	"星星棋手",
	"草莓麻薯",
	"抹茶奶盖",
}

var defaultAvatars = []string{
	"peach-cat",
	"matcha-rabbit",
	"blueberry-bear",
	"custard-chick",
	"milk-tea-puppy",
	"grape-fox",
}

type identityUser struct {
	ID        string `json:"id"`
	Nickname  string `json:"nickname"`
	Avatar    string `json:"avatar"`
	Rating    int    `json:"rating"`
	Wins      int    `json:"wins"`
	Losses    int    `json:"losses"`
	Draws     int    `json:"draws"`
	CreatedAt int64  `json:"createdAt"`
}

type identitySession struct {
	UserID    string `json:"userId"`
	ExpiresAt int64  `json:"expiresAt"`
}

type identityState struct {
	Version  int                        `json:"version"`
	Users    map[string]identityUser    `json:"users"`
	Devices  map[string]string          `json:"devices"`
	Sessions map[string]identitySession `json:"sessions"`
}

type identityStore struct {
	mu    sync.RWMutex
	path  string
	state identityState
}

type sessionResponse struct {
	User      identityUser `json:"user"`
	ExpiresAt int64        `json:"expiresAt"`
}

type updateProfileRequest struct {
	Nickname string `json:"nickname"`
}

func newIdentityStore(path string) (*identityStore, error) {
	store := &identityStore{
		path: path,
		state: identityState{
			Version:  identityStateVersion,
			Users:    make(map[string]identityUser),
			Devices:  make(map[string]string),
			Sessions: make(map[string]identitySession),
		},
	}
	if path == "" {
		return store, nil
	}

	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read identity state: %w", err)
	}
	if err := json.Unmarshal(content, &store.state); err != nil {
		return nil, fmt.Errorf("decode identity state: %w", err)
	}
	if store.state.Version != identityStateVersion {
		return nil, fmt.Errorf("unsupported identity state version %d", store.state.Version)
	}
	store.ensureMapsLocked()

	now := time.Now().UnixMilli()
	removedExpiredSession := false
	for tokenHash, session := range store.state.Sessions {
		if session.ExpiresAt <= now || store.state.Users[session.UserID].ID == "" {
			delete(store.state.Sessions, tokenHash)
			removedExpiredSession = true
		}
	}
	if removedExpiredSession {
		if err := store.saveLocked(); err != nil {
			return nil, err
		}
	}
	return store, nil
}

func (store *identityStore) getOrCreateSession(response http.ResponseWriter, request *http.Request) (sessionResponse, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	now := time.Now()
	store.ensureMapsLocked()

	if cookie, err := request.Cookie(sessionCookieName); err == nil && cookie.Value != "" {
		tokenHash := hashIdentityToken(cookie.Value)
		session, exists := store.state.Sessions[tokenHash]
		user, userExists := store.state.Users[session.UserID]
		if exists && userExists && session.ExpiresAt > now.UnixMilli() {
			store.refreshKnownDeviceCookie(response, request, user.ID, now)
			return sessionResponse{User: user, ExpiresAt: session.ExpiresAt}, nil
		}
		if exists {
			delete(store.state.Sessions, tokenHash)
		}
	}

	var user identityUser
	deviceToken := ""
	if cookie, err := request.Cookie(deviceCookieName); err == nil && cookie.Value != "" {
		if userID := store.state.Devices[hashIdentityToken(cookie.Value)]; userID != "" {
			user = store.state.Users[userID]
			if user.ID != "" {
				deviceToken = cookie.Value
			}
		}
	}

	if user.ID == "" {
		var err error
		user, deviceToken, err = store.createUserLocked(now)
		if err != nil {
			return sessionResponse{}, err
		}
	}

	sessionToken, err := newToken()
	if err != nil {
		return sessionResponse{}, fmt.Errorf("generate session token: %w", err)
	}
	expiresAt := now.Add(sessionLifetime)
	store.state.Sessions[hashIdentityToken(sessionToken)] = identitySession{
		UserID:    user.ID,
		ExpiresAt: expiresAt.UnixMilli(),
	}
	if err := store.saveLocked(); err != nil {
		return sessionResponse{}, err
	}

	setIdentityCookie(response, request, sessionCookieName, sessionToken, expiresAt)
	setIdentityCookie(response, request, deviceCookieName, deviceToken, now.Add(deviceLifetime))
	return sessionResponse{User: user, ExpiresAt: expiresAt.UnixMilli()}, nil
}

func (store *identityStore) createUserLocked(now time.Time) (identityUser, string, error) {
	userCode, err := randomString(8, roomAlphabet)
	if err != nil {
		return identityUser{}, "", fmt.Errorf("generate user id: %w", err)
	}
	deviceToken, err := newToken()
	if err != nil {
		return identityUser{}, "", fmt.Errorf("generate device token: %w", err)
	}
	nickname, err := randomIdentityDefault(defaultNicknames)
	if err != nil {
		return identityUser{}, "", fmt.Errorf("choose default nickname: %w", err)
	}
	avatar, err := randomIdentityDefault(defaultAvatars)
	if err != nil {
		return identityUser{}, "", fmt.Errorf("choose default avatar: %w", err)
	}
	user := identityUser{
		ID:        "QY" + userCode,
		Nickname:  nickname,
		Avatar:    avatar,
		Rating:    1000,
		CreatedAt: now.UnixMilli(),
	}
	store.state.Users[user.ID] = user
	store.state.Devices[hashIdentityToken(deviceToken)] = user.ID
	return user, deviceToken, nil
}

func randomIdentityDefault(options []string) (string, error) {
	if len(options) == 0 {
		return "", errors.New("default identity options are empty")
	}
	index, err := rand.Int(rand.Reader, big.NewInt(int64(len(options))))
	if err != nil {
		return "", err
	}
	return options[index.Int64()], nil
}

func (store *identityStore) updateNickname(userID, nickname string) (identityUser, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	user, exists := store.state.Users[userID]
	if !exists {
		return identityUser{}, errors.New("identity user not found")
	}
	previous := user
	user.Nickname = nickname
	store.state.Users[userID] = user
	if err := store.saveLocked(); err != nil {
		store.state.Users[userID] = previous
		return identityUser{}, err
	}
	return user, nil
}

func (store *identityStore) refreshKnownDeviceCookie(response http.ResponseWriter, request *http.Request, userID string, now time.Time) {
	cookie, err := request.Cookie(deviceCookieName)
	if err != nil || cookie.Value == "" {
		return
	}
	if store.state.Devices[hashIdentityToken(cookie.Value)] != userID {
		return
	}
	setIdentityCookie(response, request, deviceCookieName, cookie.Value, now.Add(deviceLifetime))
}

func (store *identityStore) ensureMapsLocked() {
	if store.state.Users == nil {
		store.state.Users = make(map[string]identityUser)
	}
	if store.state.Devices == nil {
		store.state.Devices = make(map[string]string)
	}
	if store.state.Sessions == nil {
		store.state.Sessions = make(map[string]identitySession)
	}
}

// saveLocked serializes every disk flush behind store.mu. It writes a complete
// snapshot to a temporary file and atomically replaces the previous state.
func (store *identityStore) saveLocked() error {
	if store.path == "" {
		return nil
	}
	content, err := json.MarshalIndent(store.state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode identity state: %w", err)
	}
	content = append(content, '\n')

	directory := filepath.Dir(store.path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create identity directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, "."+filepath.Base(store.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temporary identity state: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)

	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect temporary identity state: %w", err)
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary identity state: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary identity state: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary identity state: %w", err)
	}
	if err := os.Rename(temporaryName, store.path); err != nil {
		return fmt.Errorf("replace identity state: %w", err)
	}
	if directoryHandle, err := os.Open(directory); err == nil {
		_ = directoryHandle.Sync()
		_ = directoryHandle.Close()
	}
	return nil
}

func hashIdentityToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func setIdentityCookie(response http.ResponseWriter, request *http.Request, name, value string, expiresAt time.Time) {
	http.SetCookie(response, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  expiresAt,
		MaxAge:   max(1, int(time.Until(expiresAt).Seconds())),
		HttpOnly: true,
		Secure:   request.TLS != nil || strings.EqualFold(request.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteLaxMode,
	})
}
