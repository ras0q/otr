package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/ras0q/otr/outbound/source"
)

type Source struct {
	httpClient *http.Client
	token      string
}

var _ source.Source = (*Source)(nil)

func NewSource(token string) *Source {
	return &Source{
		httpClient: http.DefaultClient,
		token:      token,
	}
}

func (s *Source) GetLatestRelease(ctx context.Context, identity source.Identity) (source.Release, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", identity)
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
	for _, apiAsset := range body.Assets {
		downloadURL, err := url.Parse(apiAsset.BrowserDownloadURL)
		if err != nil {
			return source.Release{}, fmt.Errorf("parse asset url: %w", err)
		}
		assets = append(assets, source.Asset{
			Name: apiAsset.Name,
			Size: apiAsset.Size,
			URL:  *downloadURL,
		})
	}

	return source.Release{
		Version:       version,
		RepositoryURL: "https://github.com/" + string(identity) + ".git",
		Assets:        assets,
	}, nil
}

func (s *Source) DownloadReleaseAsset(ctx context.Context, assetURL url.URL) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL.String(), nil)
	if err != nil {
		return nil, err
	}
	if s.token != "" {
		req.Header.Set("Authorization", "Bearer "+s.token)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("download asset: %s", resp.Status)
	}
	return resp.Body, nil
}
