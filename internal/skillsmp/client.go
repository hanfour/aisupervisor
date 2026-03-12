package skillsmp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://skillsmp.com/api/v1"

// Client is an HTTP client for the SkillsMP marketplace API.
type Client struct {
	apiKey     string
	httpClient *http.Client
	baseURL    string
}

// Skill represents a skill from the SkillsMP marketplace.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Repo        string `json:"repo"`
	Stars       int    `json:"stars"`
	SkillName   string `json:"skillName"`
}

// SearchResult contains paginated search results.
type SearchResult struct {
	Skills     []Skill    `json:"skills"`
	Pagination Pagination `json:"pagination"`
}

// Pagination holds pagination metadata.
type Pagination struct {
	Page       int `json:"page"`
	Limit      int `json:"limit"`
	TotalPages int `json:"totalPages"`
	Total      int `json:"total"`
}

// NewClient creates a new SkillsMP API client.
func NewClient(apiKey string) *Client {
	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: defaultBaseURL,
	}
}

// Search performs a keyword search on the SkillsMP marketplace.
func (c *Client) Search(ctx context.Context, query string, page, limit int) (*SearchResult, error) {
	params := url.Values{}
	params.Set("q", query)
	params.Set("page", strconv.Itoa(page))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("sortBy", "stars")

	reqURL := c.baseURL + "/skills/search?" + params.Encode()
	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("skillsmp search: %w", err)
	}

	var resp struct {
		Data SearchResult `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("skillsmp search parse: %w", err)
	}
	return &resp.Data, nil
}

// AISearch performs a semantic AI search on the SkillsMP marketplace.
func (c *Client) AISearch(ctx context.Context, query string) ([]Skill, error) {
	params := url.Values{}
	params.Set("q", query)

	reqURL := c.baseURL + "/skills/ai-search?" + params.Encode()
	body, err := c.doRequest(ctx, reqURL)
	if err != nil {
		return nil, fmt.Errorf("skillsmp ai-search: %w", err)
	}

	var resp struct {
		Data struct {
			Skills []Skill `json:"skills"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("skillsmp ai-search parse: %w", err)
	}
	return resp.Data.Skills, nil
}

// ReadSkill fetches the SKILL.md content from GitHub for a given skill.
func (c *Client) ReadSkill(ctx context.Context, repo, skillName string) (string, error) {
	// Try main branch first
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/main/.skills/%s/SKILL.md", repo, skillName)
	body, err := c.doRawGet(ctx, rawURL)
	if err == nil {
		return string(body), nil
	}

	// Fallback to master branch
	rawURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/master/.skills/%s/SKILL.md", repo, skillName)
	body, err = c.doRawGet(ctx, rawURL)
	if err != nil {
		return "", fmt.Errorf("skillsmp read skill %s/%s: %w", repo, skillName, err)
	}
	return string(body), nil
}

func (c *Client) doRequest(ctx context.Context, reqURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *Client) doRawGet(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}
