package devserver

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHub_BroadcastsToSubscribers(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	// Two independent subscribers.
	r1, bodies1 := subscribe(t, srv.URL)
	defer r1.Body.Close()
	r2, bodies2 := subscribe(t, srv.URL)
	defer r2.Body.Close()

	// Wait until the hub has registered both.
	deadline := time.Now().Add(time.Second)
	for hub.Count() < 2 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.Count() != 2 {
		t.Fatalf("hub count = %d, want 2", hub.Count())
	}

	hub.Broadcast("reload")

	expect(t, bodies1, "reload")
	expect(t, bodies2, "reload")
}

func TestHub_DeadSubscriberDropped(t *testing.T) {
	hub := NewHub()
	srv := httptest.NewServer(hub.Handler())
	defer srv.Close()

	r, _ := subscribe(t, srv.URL)
	deadline := time.Now().Add(time.Second)
	for hub.Count() < 1 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}

	r.Body.Close()

	// Let the handler goroutine observe the closed connection.
	deadline = time.Now().Add(time.Second)
	for hub.Count() != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hub.Count() != 0 {
		t.Fatalf("hub did not drop closed subscriber; count=%d", hub.Count())
	}
}

// subscribe opens an SSE connection and returns the response + a channel of
// parsed "data: ..." payloads.
func subscribe(t *testing.T, url string) (*http.Response, chan string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	out := make(chan string, 4)
	go func() {
		defer close(out)
		rdr := bufio.NewReader(resp.Body)
		for {
			line, err := rdr.ReadString('\n')
			if err == io.EOF {
				return
			}
			if err != nil {
				return
			}
			if strings.HasPrefix(line, "data: ") {
				out <- strings.TrimSpace(strings.TrimPrefix(line, "data: "))
			}
		}
	}()
	return resp, out
}

func expect(t *testing.T, ch chan string, want string) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatalf("no message received for %q", want)
	}
}
