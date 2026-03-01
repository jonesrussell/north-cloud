package x

import (
	"context"
	"fmt"
	"strings"

	"github.com/jonesrussell/north-cloud/social-publisher/internal/domain"
)

// MaxTweetLength is the character limit for a single tweet.
const MaxTweetLength = 280

// urlCharacterCount is how X counts any URL regardless of actual length.
const urlCharacterCount = 23

// threadNumberPadding reserves space for thread numbering.
const threadNumberPadding = 5

// Adapter implements domain.PlatformAdapter for X (Twitter).
type Adapter struct {
	client *Client
}

// NewAdapter creates a new X adapter with the given API client.
func NewAdapter(client *Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) Name() string { return "x" }

func (a *Adapter) Capabilities() domain.PlatformCapabilities {
	return domain.PlatformCapabilities{
		SupportsImages:    true,
		SupportsThreading: true,
		SupportsMarkdown:  false,
		SupportsHTML:      false,
		MaxLength:         MaxTweetLength,
	}
}

func (a *Adapter) Transform(content domain.PublishMessage) (domain.PlatformPost, error) {
	text := buildTweetText(content)

	post := domain.PlatformPost{
		Platform: "x",
		Content:  text,
		URL:      content.URL,
		Images:   content.Images,
		Tags:     content.Tags,
	}

	// If the full body is provided and much longer, create a thread
	if content.Body != "" && len(content.Body) > MaxTweetLength*2 {
		post.Thread = splitThread(content.Summary, content.Body, content.URL)
	}

	return post, nil
}

func (a *Adapter) Validate(post domain.PlatformPost) error {
	if post.Content == "" {
		return &domain.ValidationError{Field: "content", Message: "tweet content is required"}
	}
	if len(post.Thread) == 0 && len(post.Content) > MaxTweetLength {
		return &domain.ValidationError{
			Field:   "content",
			Message: fmt.Sprintf("tweet exceeds %d characters (%d)", MaxTweetLength, len(post.Content)),
		}
	}
	return nil
}

func (a *Adapter) Publish(ctx context.Context, post domain.PlatformPost) (domain.DeliveryResult, error) {
	if a.client == nil {
		return domain.DeliveryResult{}, &domain.PermanentError{Message: "X client not configured"}
	}

	if len(post.Thread) > 0 {
		return a.client.PostThread(ctx, post.Thread)
	}
	return a.client.PostTweet(ctx, post.Content)
}

func buildTweetText(content domain.PublishMessage) string {
	text := content.Summary
	if content.URL != "" {
		textBudget := MaxTweetLength - urlCharacterCount - 1 // 23 for URL + 1 for space
		if len(text) > textBudget {
			text = text[:textBudget-3] + "..."
		}
		text = fmt.Sprintf("%s %s", text, content.URL)
	}

	if len(content.Tags) > 0 {
		hashtags := formatHashtags(content.Tags)
		if len(text)+1+len(hashtags) <= MaxTweetLength {
			text = fmt.Sprintf("%s\n%s", text, hashtags)
		}
	}

	return text
}

func formatHashtags(tags []string) string {
	hashtags := make([]string, 0, len(tags))
	for _, tag := range tags {
		cleaned := strings.ReplaceAll(tag, "-", "")
		cleaned = strings.ReplaceAll(cleaned, " ", "")
		hashtags = append(hashtags, "#"+cleaned)
	}
	return strings.Join(hashtags, " ")
}

func splitThread(summary, body, url string) []string {
	first := summary
	if url != "" {
		first = fmt.Sprintf("%s %s", summary, url)
	}

	sentences := strings.Split(body, ". ")
	thread := []string{first}

	current := ""
	for _, sentence := range sentences {
		candidate := current + sentence + ". "
		if len(candidate) > MaxTweetLength-threadNumberPadding {
			if current != "" {
				thread = append(thread, strings.TrimSpace(current))
			}
			current = sentence + ". "
		} else {
			current = candidate
		}
	}
	if current != "" {
		thread = append(thread, strings.TrimSpace(current))
	}

	return thread
}
