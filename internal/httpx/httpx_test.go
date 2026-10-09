package httpx

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	paynp "github.com/mukezhz/pay-np"
)

func TestTimeoutWrapsErrTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { time.Sleep(200 * time.Millisecond) }))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, _, err := Do(ctx, srv.Client(), paynp.Khalti, Request{Method: http.MethodGet, URL: srv.URL}); !errors.Is(err, paynp.ErrTimeout) {
		t.Fatalf("want ErrTimeout, got %v", err)
	}
}
