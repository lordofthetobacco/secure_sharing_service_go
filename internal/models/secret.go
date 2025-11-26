package models

import "time"

type Secret struct {
	ID            string    `json:"id"`
	EncryptedData []byte    `json:"-"`         // PGP encrypted
	MaxViews      int       `json:"max_views"` // e.g., 3
	CurrentViews  int       `json:"current_views"`
	ExpiresAt     time.Time `json:"expires_at"`
	CreatedAt     time.Time `json:"created_at"`
	Passphrase    string    `json:"-"` // For symmetric PGP (optional)
}
