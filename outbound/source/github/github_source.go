package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/ras0q/otr/outbound/source"
)

type Source struct {
	httpClient *http.Client
	token      string
}

func NewSource(token string) *Source {
	return &Source{
		httpClient: http.DefaultClient,
		token:      token,
	}
}

func (s *Source) GetLatestRelease(ctx context.Context, id source.Identity) (source.Release, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return source.Release{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return source.Release{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return source.Release{}, fmt.Errorf("github api: %s", resp.Status)
	}

	var body struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name               string `json:"name"`
			Size               int64  `json:"size"`
			BrowserDownloadURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return source.Release{}, fmt.Errorf("decode release: %w", err)
	}

	version := source.Version(strings.TrimPrefix(body.TagName, "v"))
	assets := make([]source.Asset, 0, len(body.Assets))
	for _, a := range body.Assets {
		u, err := url.Parse(a.BrowserDownloadURL)
		if err != nil {
			return source.Release{}, fmt.Errorf("parse asset url: %w", err)
		}
		assets = append(assets, source.Asset{
			Name: a.Name,
			Size: a.Size,
			URL:  *u,
		})
	}

	return source.Release{
		Version:       version,
		RepositoryURL: "https://github.com/" + string(id) + ".git",
		Assets:        assets,
	}, nil
}
