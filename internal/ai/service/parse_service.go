package service

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"

	"github.com/luissebastian953/stratix-core/pkg/apperror"
)

//go:embed prompts/parse.md
var parsePromptTmpl string

var parsePrompt = template.Must(template.New("parse").Parse(parsePromptTmpl))

// ParseResult holds the structured fields extracted from natural language.
// All fields except Title are optional — the client fills in any nulls.
type ParseResult struct {
	Title    string     `json:"title"`
	Type     string     `json:"type,omitempty"`
	Priority string     `json:"priority,omitempty"`
	StartAt  *time.Time `json:"start_at,omitempty"`
	EndAt    *time.Time `json:"end_at,omitempty"`
	Location string     `json:"location,omitempty"`
	Labels   []string   `json:"labels,omitempty"`
	Details  string     `json:"details,omitempty"`
}

type ParseServiceConfig struct {
	APIKey     string
	APIURL     string
	Model      string
	APIVersion string
	MaxTokens  int
	Timeout    time.Duration
}

type ParseService struct {
	cfg    ParseServiceConfig
	client *http.Client
}

func NewParseService(cfg ParseServiceConfig) *ParseService {
	return &ParseService{
		cfg:    cfg,
		client: &http.Client{Timeout: cfg.Timeout},
	}
}

// Parse sends the user's free-text input to Claude and returns structured task fields.
func (s *ParseService) Parse(ctx context.Context, input string, now time.Time) (*ParseResult, error) {
	if strings.TrimSpace(input) == "" {
		return nil, apperror.BadRequest("input is required")
	}

	prompt := buildPrompt(input, now)
	body, err := s.callClaude(ctx, prompt)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("claude: %w", err))
	}

	result, err := parseClaudeResponse(body)
	if err != nil {
		return nil, apperror.Internal(fmt.Errorf("parse claude response: %w", err))
	}

	return result, nil
}

// ── Prompt ────────────────────────────────────────────────────────────────────

func buildPrompt(input string, now time.Time) string {
	var buf bytes.Buffer
	_ = parsePrompt.Execute(&buf, struct {
		Now   string
		Input string
	}{
		Now:   now.UTC().Format("Monday, 2 January 2006 15:04 UTC"),
		Input: input,
	})
	return buf.String()
}

// ── Anthropic API ─────────────────────────────────────────────────────────────

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (s *ParseService) callClaude(ctx context.Context, prompt string) (string, error) {
	reqBody, err := json.Marshal(anthropicRequest{
		Model:     s.cfg.Model,
		MaxTokens: s.cfg.MaxTokens,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.APIURL, bytes.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", s.cfg.APIKey)
	req.Header.Set("anthropic-version", s.cfg.APIVersion)

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var apiResp anthropicResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		msg := "unexpected status " + resp.Status
		if apiResp.Error != nil {
			msg = apiResp.Error.Message
		}
		return "", errors.New(msg)
	}

	if len(apiResp.Content) == 0 || apiResp.Content[0].Text == "" {
		return "", errors.New("empty response from Claude")
	}

	return apiResp.Content[0].Text, nil
}

// ── Response parsing ──────────────────────────────────────────────────────────

// rawParseResult mirrors ParseResult but uses string for times so we can
// parse them ourselves rather than relying on json.Unmarshal's time handling.
type rawParseResult struct {
	Title    string   `json:"title"`
	Type     string   `json:"type"`
	Priority string   `json:"priority"`
	StartAt  string   `json:"start_at"`
	EndAt    string   `json:"end_at"`
	Location string   `json:"location"`
	Labels   []string `json:"labels"`
	Details  string   `json:"details"`
}

func parseClaudeResponse(text string) (*ParseResult, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var raw rawParseResult
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON from Claude: %w", err)
	}

	if strings.TrimSpace(raw.Title) == "" {
		return nil, errors.New("Claude returned no title")
	}

	result := &ParseResult{
		Title:    strings.TrimSpace(raw.Title),
		Type:     raw.Type,
		Priority: raw.Priority,
		Location: raw.Location,
		Labels:   raw.Labels,
		Details:  raw.Details,
	}

	if raw.StartAt != "" {
		t, err := time.Parse(time.RFC3339, raw.StartAt)
		if err == nil {
			utc := t.UTC()
			result.StartAt = &utc
		}
	}
	if raw.EndAt != "" {
		t, err := time.Parse(time.RFC3339, raw.EndAt)
		if err == nil {
			utc := t.UTC()
			result.EndAt = &utc
		}
	}

	return result, nil
}
