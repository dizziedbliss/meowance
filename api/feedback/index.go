// Package handler is the Vercel Go function behind POST /api/feedback.
package handler

import (
	"net/http"

	"github.com/dizziedbliss/meowance/pkg/api"
)

// RESEND_API_KEY, FEEDBACK_FROM and FEEDBACK_TO come from the project's env;
// without them the box answers 503 feedback_off.
var feedback = api.Feedback(api.ResendFromEnv())

// Handler is the entry point Vercel calls.
func Handler(w http.ResponseWriter, r *http.Request) { feedback(w, r) }
