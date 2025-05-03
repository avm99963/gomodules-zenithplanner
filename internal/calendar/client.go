package calendar

import (
	"context"
	"fmt"
	"gomodules.avm99963.com/zenithplanner/internal/config"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// NewService creates an authenticated Google Calendar service client.
// It uses the provided refresh token to obtain access tokens automatically.
func NewService(ctx context.Context, googleCfg config.GoogleConfig) (*calendar.Service, error) {
	if googleCfg.RefreshToken == "" {
		return nil, fmt.Errorf("google refresh token is required")
	}

	// Use ClientID and ClientSecret if provided (often needed for refresh mechanism)
	// Otherwise, the library might attempt other discovery methods if run on GCP etc.
	oauthConfig := &oauth2.Config{
		ClientID:     googleCfg.ClientID,
		ClientSecret: googleCfg.ClientSecret,
		Endpoint:     google.Endpoint,
		Scopes: []string{ // Ensure scopes match those requested by CLI
			calendar.CalendarReadonlyScope,
			calendar.CalendarEventsScope,
		},
	}

	token := &oauth2.Token{
		RefreshToken: googleCfg.RefreshToken,
	}
	tokenSource := oauthConfig.TokenSource(ctx, token)

	srv, err := calendar.NewService(ctx, option.WithTokenSource(tokenSource))
	if err != nil {
		return nil, fmt.Errorf("unable to retrieve Calendar client: %w", err)
	}

	return srv, nil
}
