package game

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestSessionSurvivesServerRestart(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	firstHandler, err := NewServerWithDataFile(os.DirFS("../.."), statePath)
	if err != nil {
		t.Fatal(err)
	}
	firstServer := httptest.NewServer(firstHandler)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar}
	firstSession := requestIdentitySession(t, client, firstServer.URL)
	firstURL, err := url.Parse(firstServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	var rawSessionToken string
	var rawDeviceToken string
	for _, cookie := range jar.Cookies(firstURL) {
		switch cookie.Name {
		case sessionCookieName:
			rawSessionToken = cookie.Value
		case deviceCookieName:
			rawDeviceToken = cookie.Value
		}
	}
	if rawSessionToken == "" || rawDeviceToken == "" {
		t.Fatal("identity cookies were not issued")
	}
	firstServer.Close()

	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(content, []byte(rawSessionToken)) || bytes.Contains(content, []byte(rawDeviceToken)) {
		t.Fatal("raw identity tokens must not be persisted")
	}

	secondHandler, err := NewServerWithDataFile(os.DirFS("../.."), statePath)
	if err != nil {
		t.Fatal(err)
	}
	secondServer := httptest.NewServer(secondHandler)
	defer secondServer.Close()

	secondSession := requestIdentitySession(t, client, secondServer.URL)
	if secondSession.User.ID != firstSession.User.ID {
		t.Fatalf("user changed after restart: %q != %q", secondSession.User.ID, firstSession.User.ID)
	}
	if secondSession.ExpiresAt != firstSession.ExpiresAt {
		t.Fatalf("session was replaced after restart: %d != %d", secondSession.ExpiresAt, firstSession.ExpiresAt)
	}
}

func TestConcurrentSessionWritesKeepValidState(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "state.json")
	handler, err := NewServerWithDataFile(os.DirFS("../.."), statePath)
	if err != nil {
		t.Fatal(err)
	}
	testServer := httptest.NewServer(handler)
	defer testServer.Close()

	const clients = 12
	var waitGroup sync.WaitGroup
	errors := make(chan error, clients)
	for index := 0; index < clients; index++ {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			response, err := http.Get(testServer.URL + "/api/session")
			if err != nil {
				errors <- err
				return
			}
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode != http.StatusOK {
				errors <- &statusError{status: response.StatusCode}
			}
		}()
	}
	waitGroup.Wait()
	close(errors)
	for err := range errors {
		t.Fatal(err)
	}

	content, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	var state identityState
	if err := json.Unmarshal(content, &state); err != nil {
		t.Fatalf("identity state is not valid JSON: %v", err)
	}
	if len(state.Users) != clients || len(state.Devices) != clients || len(state.Sessions) != clients {
		t.Fatalf(
			"unexpected persisted counts: users=%d devices=%d sessions=%d",
			len(state.Users),
			len(state.Devices),
			len(state.Sessions),
		)
	}
}

type statusError struct {
	status int
}

func (err *statusError) Error() string {
	return http.StatusText(err.status)
}

func requestIdentitySession(t *testing.T, client *http.Client, serverURL string) sessionResponse {
	t.Helper()
	response, err := client.Get(serverURL + "/api/session")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("session returned %d", response.StatusCode)
	}
	var session sessionResponse
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	return session
}
