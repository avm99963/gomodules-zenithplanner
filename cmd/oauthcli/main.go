package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
)

const redirectURL = "http://localhost:8080/oauth2callback"

func main() {
	fmt.Println("Starting ZenithPlanner OAuth CLI Tool...")

	config := loadCredentialsAndConfig()
	authURL := generateAuthURL(config)
	authCode := getUserAuthorization(authURL)
	token := exchangeCodeForToken(config, authCode)
	displayToken(token)
}

// loadCredentialsAndConfig loads Google Cloud credentials from environment
// variables and configures the OAuth2 client.
func loadCredentialsAndConfig() *oauth2.Config {
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if googleClientID == "" || googleClientSecret == "" {
		log.Fatal("Error: GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET environment variables must be set.")
	}

	return &oauth2.Config{
		ClientID:     googleClientID,
		ClientSecret: googleClientSecret,
		RedirectURL:  redirectURL,
		Scopes: []string{
			calendar.CalendarReadonlyScope, // Read access
			calendar.CalendarEventsScope,   // Write/Delete access
		},
		Endpoint: google.Endpoint,
	}
}

// generateAuthURL creates the URL for the user to visit for authorization.
func generateAuthURL(config *oauth2.Config) string {
	// We use "offline" access type to get a refresh token.
	return config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
}

// getUserAuthorization prompts the user to visit the auth URL and enter the code.
func getUserAuthorization(authURL string) string {
	fmt.Printf("--------------------------------------------------\n")
	fmt.Printf("1. Go to the following link in your browser:\n\n%v\n\n", authURL)
	fmt.Printf("2. Grant ZenithPlanner access to your Google Calendar.\n")
	fmt.Printf("3. After authorization, Google will redirect you to a URL like:\n")
	fmt.Printf("   %s?state=state-token&code=XXXXXXXXX\n", redirectURL)
	fmt.Printf("4. Copy the 'code' value from that URL.\n")
	fmt.Printf("--------------------------------------------------\n")

	var authCode string
	fmt.Print("Enter the authorization code: ")
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}
	return authCode
}

// exchangeCodeForToken exchanges the authorization code for OAuth2 tokens.
func exchangeCodeForToken(config *oauth2.Config, authCode string) *oauth2.Token {
	ctx := context.Background()
	tok, err := config.Exchange(ctx, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// displayToken shows the obtained tokens to the user, highlighting the refresh token.
func displayToken(token *oauth2.Token) {
	fmt.Printf("\n--------------------------------------------------\n")
	fmt.Printf("OAuth flow successful!\n")
	fmt.Printf("\nAccess Token (short-lived):\n%s\n", token.AccessToken)
	if token.RefreshToken != "" {
		fmt.Printf("\nRefresh Token (long-lived - STORE THIS SECURELY!):\n%s\n", token.RefreshToken)
		fmt.Printf("\nSet this Refresh Token as the GOOGLE_REFRESH_TOKEN environment variable for the backend service.\n")
	} else {
		fmt.Printf("\nWARNING: No Refresh Token received. Did you already authorize this app?\n")
		fmt.Printf("You might need to revoke access in your Google Account settings and run this again.\n")
		fmt.Printf("Ensure 'AccessTypeOffline' is used.\n")
	}
	fmt.Printf("\nToken details (expiry, etc.):\n")
	tokenJSON, _ := json.MarshalIndent(token, "", "  ")
	fmt.Println(string(tokenJSON))
	fmt.Printf("--------------------------------------------------\n")
}
