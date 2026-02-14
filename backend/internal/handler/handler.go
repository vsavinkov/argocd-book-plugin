package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/behavox/argocd-book-plugin/internal/k8s"
)

const (
	headerAppName    = "Argocd-Application-Name"
	headerUsername   = "Argocd-Username"
	headerUserGroups = "Argocd-User-Groups"

	adminGroup = "admin"
)

type statusResponse struct {
	Booked   bool   `json:"booked"`
	BookedBy string `json:"bookedBy,omitempty"`
	BookedAt string `json:"bookedAt,omitempty"`
}

type errorResponse struct {
	Error string `json:"error"`
}

// Handler provides HTTP handlers for the booking API.
type Handler struct {
	client k8s.Client
}

// New creates a new Handler with the given K8s client.
func New(client k8s.Client) *Handler {
	return &Handler{client: client}
}

// RegisterRoutes registers all booking API routes on the given mux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/status", h.Status)
	mux.HandleFunc("POST /api/book", h.Book)
	mux.HandleFunc("POST /api/unbook", h.Unbook)
	mux.HandleFunc("GET /api/list", h.List)
	mux.HandleFunc("GET /healthz", h.Healthz)
}

// parseAppHeader parses the "Argocd-Application-Name" header in the format "namespace:appname".
func parseAppHeader(r *http.Request) (namespace, appName string, ok bool) {
	raw := r.Header.Get(headerAppName)
	if raw == "" {
		return "", "", false
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	return parts[0], parts[1], true
}

func isAdmin(r *http.Request) bool {
	groups := r.Header.Get(headerUserGroups)
	for _, g := range strings.Split(groups, ",") {
		if strings.TrimSpace(g) == adminGroup {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// Status returns the booking status of an application.
func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	ns, app, ok := parseAppHeader(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing or invalid Argocd-Application-Name header (expected namespace:appname)")
		return
	}

	bookedBy, bookedAt, err := h.client.GetBookingStatus(r.Context(), ns, app)
	if err != nil {
		log.Printf("error getting booking status for %s/%s: %v", ns, app, err)
		writeError(w, http.StatusInternalServerError, "failed to get booking status")
		return
	}

	resp := statusResponse{Booked: bookedBy != ""}
	if bookedBy != "" {
		resp.BookedBy = bookedBy
		resp.BookedAt = bookedAt.UTC().Format("2006-01-02T15:04:05Z")
	}
	writeJSON(w, http.StatusOK, resp)
}

// Book books an application for the requesting user.
func (h *Handler) Book(w http.ResponseWriter, r *http.Request) {
	ns, app, ok := parseAppHeader(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing or invalid Argocd-Application-Name header (expected namespace:appname)")
		return
	}

	username := r.Header.Get(headerUsername)
	if username == "" {
		writeError(w, http.StatusBadRequest, "missing Argocd-Username header")
		return
	}

	err := h.client.BookApp(r.Context(), ns, app, username)
	if err != nil {
		if strings.Contains(err.Error(), "conflict:") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		log.Printf("error booking %s/%s for %s: %v", ns, app, username, err)
		writeError(w, http.StatusInternalServerError, "failed to book application")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "booked"})
}

// Unbook unbooks an application.
func (h *Handler) Unbook(w http.ResponseWriter, r *http.Request) {
	ns, app, ok := parseAppHeader(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "missing or invalid Argocd-Application-Name header (expected namespace:appname)")
		return
	}

	username := r.Header.Get(headerUsername)
	if username == "" {
		writeError(w, http.StatusBadRequest, "missing Argocd-Username header")
		return
	}

	err := h.client.UnbookApp(r.Context(), ns, app, username, isAdmin(r))
	if err != nil {
		if strings.Contains(err.Error(), "forbidden:") {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		log.Printf("error unbooking %s/%s by %s: %v", ns, app, username, err)
		writeError(w, http.StatusInternalServerError, "failed to unbook application")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "unbooked"})
}

// List returns all currently booked applications.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	ns := r.URL.Query().Get("namespace")
	if ns == "" {
		ns = "argocd"
	}

	bookings, err := h.client.ListBookings(r.Context(), ns)
	if err != nil {
		log.Printf("error listing bookings in %s: %v", ns, err)
		writeError(w, http.StatusInternalServerError, "failed to list bookings")
		return
	}

	if bookings == nil {
		bookings = []k8s.Booking{}
	}
	writeJSON(w, http.StatusOK, bookings)
}

// Healthz is a simple health check endpoint.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok"))
}
