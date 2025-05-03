package handler

import (
	"log"
	"net/http"
	"gomodules.avm99963.com/zenithplanner/internal/config"
	"gomodules.avm99963.com/zenithplanner/internal/sync"
)

const webhookPath = "/webhook/calendar"

// WebhookHandler holds dependencies for handling webhook requests.
type WebhookHandler struct {
	syncer            *sync.Syncer
	cfg               *config.Config
	verificationToken string
}

// NewWebhookHandler creates a new handler.
func NewWebhookHandler(syncer *sync.Syncer, cfg *config.Config) *WebhookHandler {
	return &WebhookHandler{
		syncer:            syncer,
		cfg:               cfg,
		verificationToken: cfg.Google.WebhookVerificationToken,
	}
}

// RegisterWebhookRoute registers the webhook handler with an HTTP ServeMux.
func RegisterWebhookRoute(mux *http.ServeMux, handler *WebhookHandler) {
	log.Printf("Registering webhook handler at path: %s", webhookPath)
	mux.HandleFunc(webhookPath, handler.HandleWebhook) // Use method HandleWebhook
}

// HandleWebhook processes incoming Google Calendar push notifications.
func (h *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		log.Printf("Webhook received non-POST request: %s", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// --- Validate Headers ---
	channelToken := r.Header.Get("X-Goog-Channel-Token")
	channelState := r.Header.Get("X-Goog-Resource-State")

	log.Printf("Received webhook notification: State=%s", channelState)

	if h.verificationToken == "" {
		log.Println("Warning: WEBHOOK_VERIFICATION_TOKEN is not configured. Skipping token validation.")
	} else if channelToken != h.verificationToken {
		log.Printf("Webhook validation failed: Invalid token received ('%s' != expected)", channelToken)
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check if it's a state indicating changes ('exists' or 'not_exists')
	// Ignore the initial 'sync' message.
	if channelState != "exists" && channelState != "not_exists" {
		log.Printf("Ignoring webhook notification state: %s", channelState)
		w.WriteHeader(http.StatusOK)
		return
	}

	log.Println("Webhook notification acknowledged, requesting sync...")
	h.syncer.RequestSync()

	w.WriteHeader(http.StatusOK)
}
