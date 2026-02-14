package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/behavox/argocd-book-plugin/internal/k8s"
)

// mockClient implements k8s.Client for testing.
type mockClient struct {
	bookings map[string]*k8s.Booking // key: "namespace/appName"
}

func newMockClient() *mockClient {
	return &mockClient{bookings: make(map[string]*k8s.Booking)}
}

func (m *mockClient) key(ns, app string) string { return ns + "/" + app }

func (m *mockClient) GetBookingStatus(_ context.Context, namespace, appName string) (string, time.Time, error) {
	b, ok := m.bookings[m.key(namespace, appName)]
	if !ok || b.BookedBy == "" {
		return "", time.Time{}, nil
	}
	t, _ := time.Parse(time.RFC3339, b.BookedAt)
	return b.BookedBy, t, nil
}

func (m *mockClient) BookApp(_ context.Context, namespace, appName, username string) error {
	k := m.key(namespace, appName)
	if b, ok := m.bookings[k]; ok && b.BookedBy != "" && b.BookedBy != username {
		return fmt.Errorf("conflict: application already booked by %s", b.BookedBy)
	}
	m.bookings[k] = &k8s.Booking{
		AppName:   appName,
		Namespace: namespace,
		BookedBy:  username,
		BookedAt:  time.Now().UTC().Format(time.RFC3339),
	}
	return nil
}

func (m *mockClient) UnbookApp(_ context.Context, namespace, appName, username string, isAdmin bool) error {
	k := m.key(namespace, appName)
	b, ok := m.bookings[k]
	if !ok || b.BookedBy == "" {
		return nil
	}
	if b.BookedBy != username && !isAdmin {
		return fmt.Errorf("forbidden: application is booked by %s, only they or an admin can unbook", b.BookedBy)
	}
	delete(m.bookings, k)
	return nil
}

func (m *mockClient) ListBookings(_ context.Context, namespace string) ([]k8s.Booking, error) {
	var result []k8s.Booking
	for _, b := range m.bookings {
		if b.Namespace == namespace {
			result = append(result, *b)
		}
	}
	return result, nil
}

func setupHandler() (*Handler, *mockClient, *http.ServeMux) {
	mc := newMockClient()
	h := New(mc)
	mux := http.NewServeMux()
	h.RegisterRoutes(mux)
	return h, mc, mux
}

func TestStatus_NotBooked(t *testing.T) {
	_, _, mux := setupHandler()

	req := httptest.NewRequest("GET", "/api/status", nil)
	req.Header.Set(headerAppName, "argocd:my-app")
	w := httptest.NewRecorder()

	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp statusResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Booked {
		t.Fatal("expected not booked")
	}
}

func TestStatus_MissingHeader(t *testing.T) {
	_, _, mux := setupHandler()

	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestBook_Success(t *testing.T) {
	_, _, mux := setupHandler()

	req := httptest.NewRequest("POST", "/api/book", nil)
	req.Header.Set(headerAppName, "argocd:my-app")
	req.Header.Set(headerUsername, "alice")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify status
	req2 := httptest.NewRequest("GET", "/api/status", nil)
	req2.Header.Set(headerAppName, "argocd:my-app")
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	var resp statusResponse
	json.NewDecoder(w2.Body).Decode(&resp)
	if !resp.Booked || resp.BookedBy != "alice" {
		t.Fatalf("expected booked by alice, got %+v", resp)
	}
}

func TestBook_Conflict(t *testing.T) {
	_, mc, mux := setupHandler()

	// Pre-book as alice
	mc.BookApp(context.Background(), "argocd", "my-app", "alice")

	// Bob tries to book
	req := httptest.NewRequest("POST", "/api/book", nil)
	req.Header.Set(headerAppName, "argocd:my-app")
	req.Header.Set(headerUsername, "bob")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnbook_ByBooker(t *testing.T) {
	_, mc, mux := setupHandler()

	mc.BookApp(context.Background(), "argocd", "my-app", "alice")

	req := httptest.NewRequest("POST", "/api/unbook", nil)
	req.Header.Set(headerAppName, "argocd:my-app")
	req.Header.Set(headerUsername, "alice")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnbook_ByOtherUser_Forbidden(t *testing.T) {
	_, mc, mux := setupHandler()

	mc.BookApp(context.Background(), "argocd", "my-app", "alice")

	req := httptest.NewRequest("POST", "/api/unbook", nil)
	req.Header.Set(headerAppName, "argocd:my-app")
	req.Header.Set(headerUsername, "bob")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnbook_ByAdmin(t *testing.T) {
	_, mc, mux := setupHandler()

	mc.BookApp(context.Background(), "argocd", "my-app", "alice")

	req := httptest.NewRequest("POST", "/api/unbook", nil)
	req.Header.Set(headerAppName, "argocd:my-app")
	req.Header.Set(headerUsername, "bob")
	req.Header.Set(headerUserGroups, "developers, admin")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestList_Empty(t *testing.T) {
	_, _, mux := setupHandler()

	req := httptest.NewRequest("GET", "/api/list?namespace=argocd", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var bookings []k8s.Booking
	json.NewDecoder(w.Body).Decode(&bookings)
	if len(bookings) != 0 {
		t.Fatalf("expected empty list, got %d items", len(bookings))
	}
}

func TestList_WithBookings(t *testing.T) {
	_, mc, mux := setupHandler()

	mc.BookApp(context.Background(), "argocd", "app1", "alice")
	mc.BookApp(context.Background(), "argocd", "app2", "bob")

	req := httptest.NewRequest("GET", "/api/list?namespace=argocd", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var bookings []k8s.Booking
	json.NewDecoder(w.Body).Decode(&bookings)
	if len(bookings) != 2 {
		t.Fatalf("expected 2 bookings, got %d", len(bookings))
	}
}

func TestBook_MissingUsername(t *testing.T) {
	_, _, mux := setupHandler()

	req := httptest.NewRequest("POST", "/api/book", nil)
	req.Header.Set(headerAppName, "argocd:my-app")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestHealthz(t *testing.T) {
	_, _, mux := setupHandler()

	req := httptest.NewRequest("GET", "/healthz", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
