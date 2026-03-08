package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
)

// UserInfo is the normalized user identity from an OAuth provider.
type UserInfo struct {
	ID        string
	Name      string
	Email     string
	AvatarURL string
}

// FetchGoogleUser calls Google's userinfo endpoint with the given token.
func FetchGoogleUser(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google userinfo: status %d", resp.StatusCode)
	}

	var data struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("google userinfo decode: %w", err)
	}

	return &UserInfo{
		ID:        data.ID,
		Name:      data.Name,
		Email:     data.Email,
		AvatarURL: data.Picture,
	}, nil
}

// FetchGitHubUser calls GitHub's user API with the given token.
func FetchGitHubUser(ctx context.Context, token *oauth2.Token) (*UserInfo, error) {
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(token))
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github user: status %d", resp.StatusCode)
	}

	var data struct {
		ID        int    `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("github user decode: %w", err)
	}

	name := data.Name
	if name == "" {
		name = data.Login
	}

	return &UserInfo{
		ID:        fmt.Sprintf("%d", data.ID),
		Name:      name,
		Email:     data.Email,
		AvatarURL: data.AvatarURL,
	}, nil
}
