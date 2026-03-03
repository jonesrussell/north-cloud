package x

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

const (
	apiBaseURL = "https://api.x.com/2"
	// defaultRateLimitCooldown is the default wait when rate-limited without a Retry-After header.
	defaultRateLimitCooldown = 15 * time.Minute
)

// Client handles HTTP communication with the X API v2.
type Client struct {
	httpClient  *http.Client
	bearerToken string
}

// NewClient creates a new X API client with the given bearer token.
func NewClient(bearerToken string) *Client {
	return &Client{
		httpClient:  &http.Client{},
		bearerToken: bearerToken,
	}
}

type tweetRequest struct {
	Text  string   `json:"text"`
	Reply *replyTo `json:"reply,omitempty"`
}

type replyTo struct {
	InReplyToTweetID string `json:"in_reply_to_tweet_id"`
}

type tweetResponse struct {
	Data struct {
		ID   string `json:"id"`
		Text string `json:"text"`
	} `json:"data"`
	Errors []struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"errors"`
}

// PostTweet publishes a single tweet.
func (c *Client) PostTweet(ctx context.Context, text string) (domain.DeliveryResult, error) {
	return c.postTweet(ctx, text, "")
}

// PostThread publishes a series of tweets as a reply chain.
func (c *Client) PostThread(ctx context.Context, tweets []string) (domain.DeliveryResult, error) {
	if len(tweets) == 0 {
		return domain.DeliveryResult{}, &domain.ValidationError{Field: "thread", Message: "empty thread"}
	}

	result, err := c.postTweet(ctx, tweets[0], "")
	if err != nil {
		return result, err
	}

	lastID := result.PlatformID
	for _, tweet := range tweets[1:] {
		result, err = c.postTweet(ctx, tweet, lastID)
		if err != nil {
			return result, err // partial thread posted
		}
		lastID = result.PlatformID
	}

	return result, nil
}

func (c *Client) postTweet(ctx context.Context, text, replyToID string) (domain.DeliveryResult, error) {
	reqBody := tweetRequest{Text: text}
	if replyToID != "" {
		reqBody.Reply = &replyTo{InReplyToTweetID: replyToID}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return domain.DeliveryResult{}, fmt.Errorf("marshaling tweet request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/tweets", bytes.NewReader(body))
	if err != nil {
		return domain.DeliveryResult{}, &domain.TransientError{Message: err.Error()}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.bearerToken)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return domain.DeliveryResult{}, &domain.TransientError{Message: err.Error()}
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return domain.DeliveryResult{}, &domain.TransientError{Message: fmt.Sprintf("reading response: %s", err)}
	}

	if apiErr := classifyHTTPError(resp.StatusCode, respBody); apiErr != nil {
		return domain.DeliveryResult{}, apiErr
	}

	var tweetResp tweetResponse
	if err := json.Unmarshal(respBody, &tweetResp); err != nil {
		return domain.DeliveryResult{}, &domain.TransientError{Message: "failed to parse X API response"}
	}

	return domain.DeliveryResult{
		PlatformID:  tweetResp.Data.ID,
		PlatformURL: fmt.Sprintf("https://x.com/i/status/%s", tweetResp.Data.ID),
	}, nil
}

func classifyHTTPError(statusCode int, respBody []byte) error {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return nil
	case statusCode == http.StatusTooManyRequests:
		return &domain.RateLimitError{
			Message:    "X API rate limit exceeded",
			RetryAfter: defaultRateLimitCooldown,
		}
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return &domain.AuthError{Message: "X API authentication failed"}
	case statusCode >= http.StatusInternalServerError:
		return &domain.TransientError{
			Message: fmt.Sprintf("X API server error: %d", statusCode),
		}
	default:
		return &domain.PermanentError{
			Message:  "X API client error",
			Code:     fmt.Sprintf("%d", statusCode),
			Response: string(respBody),
		}
	}
}
