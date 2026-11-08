// Package handler is the Vercel Go function behind POST /api/attendance.
// The logic lives in internal/api so the local dev server runs the exact same code.
package handler

import (
	"net/http"

	"github.com/dizziedbliss/meowance/internal/api"
	"github.com/dizziedbliss/meowance/internal/portal"
)

var attendance = api.Attendance(portal.NewClient())

// Handler is the entry point Vercel calls.
func Handler(w http.ResponseWriter, r *http.Request) { attendance(w, r) }
