package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"bx.share/config"
	"bx.share/internal/crypto"
	"bx.share/internal/models"
	"bx.share/internal/store"
	"bx.share/web"

	"github.com/go-chi/chi/v5"
)

type Handler struct {
	store  store.Store
	config *config.Config
}

func NewHandler(s store.Store, cfg *config.Config) *Handler {
	return &Handler{
		store:  s,
		config: cfg,
	}
}

type CreateRequest struct {
	Content    string `json:"content"`
	MaxViews   int    `json:"max_views,omitempty"`
	TTLMinutes int    `json:"ttl_minutes,omitempty"`
}

type CreateResponse struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	ExpiresAt time.Time `json:"expires_at"`
	MaxViews  int       `json:"max_views"`
}

type RevealResponse struct {
	Content        string `json:"content"`
	ViewsRemaining int    `json:"views_remaining"`
}

type StatusResponse struct {
	ID             string    `json:"id"`
	Exists         bool      `json:"exists"`
	Expired        bool      `json:"expired"`
	ViewsRemaining int       `json:"views_remaining,omitempty"`
	ExpiresAt      time.Time `json:"expires_at,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.json(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) CreateSecret(w http.ResponseWriter, r *http.Request) {
	var req CreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.error(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Content == "" {
		h.error(w, http.StatusBadRequest, "content is required")
		return
	}

	maxViews := clamp(
		req.MaxViews,
		h.config.Secrets.DefaultViews,
		h.config.Secrets.MaxViews,
	)

	ttl := clampDuration(
		time.Duration(req.TTLMinutes)*time.Minute,
		h.config.Secrets.DefaultTTL,
		h.config.Secrets.MaxTTL,
	)

	id := crypto.GenerateID()
	passphrase := crypto.GeneratePassphrase()

	encrypted, err := crypto.Encrypt([]byte(req.Content), passphrase)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "encryption failed")
		return
	}

	secret := &models.Secret{
		ID:            id,
		EncryptedData: encrypted,
		Passphrase:    passphrase,
		MaxViews:      maxViews,
		CurrentViews:  0,
		ExpiresAt:     time.Now().Add(ttl),
		CreatedAt:     time.Now(),
	}

	if err := h.store.Save(r.Context(), secret); err != nil {
		h.error(w, http.StatusInternalServerError, "failed to save secret")
		return
	}

	url := h.config.Server.BaseURL + "/s/" + id + "#" + passphrase

	h.json(w, http.StatusCreated, CreateResponse{
		ID:        id,
		URL:       url,
		ExpiresAt: secret.ExpiresAt,
		MaxViews:  maxViews,
	})
}

func (h *Handler) RevealSecret(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	passphrase := r.URL.Query().Get("passphrase")

	if passphrase == "" {
		h.error(w, http.StatusBadRequest, "passphrase is required")
		return
	}

	secret, err := h.store.Get(r.Context(), id)
	if err != nil {
		h.handleStoreError(w, err)
		return
	}

	if passphrase != secret.Passphrase {
		h.error(w, http.StatusForbidden, "invalid passphrase")
		return
	}

	currentViews, err := h.store.IncrementViews(r.Context(), id)
	if err != nil {
		h.handleStoreError(w, err)
		return
	}

	content, err := crypto.Decrypt(secret.EncryptedData, passphrase)
	if err != nil {
		h.error(w, http.StatusInternalServerError, "decryption failed")
		return
	}

	h.json(w, http.StatusOK, RevealResponse{
		Content:        string(content),
		ViewsRemaining: secret.MaxViews - currentViews,
	})
}

func (h *Handler) GetStatus(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	secret, err := h.store.Get(r.Context(), id)
	if err != nil {
		status := StatusResponse{ID: id, Exists: false}
		if errors.Is(err, store.ErrExpired) {
			status.Expired = true
		}
		h.json(w, http.StatusOK, status)
		return
	}

	h.json(w, http.StatusOK, StatusResponse{
		ID:             id,
		Exists:         true,
		Expired:        false,
		ViewsRemaining: secret.MaxViews - secret.CurrentViews,
		ExpiresAt:      secret.ExpiresAt,
	})
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, "index.html")
}

func (h *Handler) RevealPage(w http.ResponseWriter, r *http.Request) {
	h.serveFile(w, "reveal.html")
}

func (h *Handler) serveFile(w http.ResponseWriter, filename string) {
	content, err := web.GetFile(filename)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	contentType := "text/html; charset=utf-8"
	w.Header().Set("Content-Type", contentType)
	w.Write(content)
}

func (h *Handler) json(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) error(w http.ResponseWriter, status int, message string) {
	h.json(w, status, ErrorResponse{Error: message})
}

func (h *Handler) handleStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		h.error(w, http.StatusNotFound, "secret not found")
	case errors.Is(err, store.ErrExpired):
		h.error(w, http.StatusGone, "secret has expired")
	case errors.Is(err, store.ErrMaxViews):
		h.error(w, http.StatusGone, "secret has reached maximum views")
	default:
		h.error(w, http.StatusInternalServerError, "internal error")
	}
}

func clamp(val, defaultVal, maxVal int) int {
	if val <= 0 {
		return defaultVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}

func clampDuration(val, defaultVal, maxVal time.Duration) time.Duration {
	if val <= 0 {
		return defaultVal
	}
	if val > maxVal {
		return maxVal
	}
	return val
}
