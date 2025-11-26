package api

import (
	"time"

	"secure.share/config"
	"secure.share/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func SetupRouter(s store.Store, cfg *config.Config) *chi.Mux {
	h := NewHandler(s, cfg)

	r := chi.NewRouter()

	// Global middleware
	r.Use(middleware.RealIP)
	r.Use(RequestID)
	r.Use(Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// CORS
	r.Use(CORS(CORSConfig{
		AllowedOrigins: []string{"127.0.0.1"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "X-Request-ID"},
		MaxAge:         86400,
	}))

	// Health
	r.Get("/health", h.Health)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Apply rate limiting if enabled
		if cfg.RateLimit.Enabled {
			apiLimiter := NewRateLimiter(cfg.RateLimit.RequestsPerMin, time.Minute)
			revealLimiter := NewRateLimiter(cfg.RateLimit.RevealPerMin, time.Minute)

			r.Use(apiLimiter.Middleware)
			r.Use(JSONOnly)

			r.Route("/secrets", func(r chi.Router) {
				r.Post("/", h.CreateSecret)
				r.With(revealLimiter.Middleware).Get("/{id}", h.RevealSecret)
				r.Get("/{id}/status", h.GetStatus)
			})
		} else {
			r.Use(JSONOnly)

			r.Route("/secrets", func(r chi.Router) {
				r.Post("/", h.CreateSecret)
				r.Get("/{id}", h.RevealSecret)
				r.Get("/{id}/status", h.GetStatus)
			})
		}
	})

	// Frontend
	r.Get("/", h.Index)
	r.Get("/s/{id}", h.RevealPage)

	return r
}
