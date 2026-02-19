package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/domain"
	"github.com/jonesrussell/north-cloud/click-tracker/internal/storage"
	"github.com/north-cloud/infrastructure/clickurl"
	infralogger "github.com/north-cloud/infrastructure/logger"
)

// errMissingParams is returned when required query parameters are absent or unparseable.
var errMissingParams = errors.New("missing required parameters (q, r, p, t, u, sig)")

// uaHashLength is the number of hex characters used for the truncated user-agent hash.
const uaHashLength = 12

// defaultPage is the page number used when the pg parameter is absent or invalid.
const defaultPage = 1

// ClickHandler handles click redirect requests.
type ClickHandler struct {
	signer *clickurl.Signer
	buffer *storage.Buffer
	logger infralogger.Logger
	maxAge time.Duration
}

// NewClickHandler creates a ClickHandler with the given dependencies.
func NewClickHandler(
	signer *clickurl.Signer,
	buffer *storage.Buffer,
	log infralogger.Logger,
	maxAge time.Duration,
) *ClickHandler {
	return &ClickHandler{
		signer: signer,
		buffer: buffer,
		logger: log,
		maxAge: maxAge,
	}
}

// HandleClick validates the signature, logs the event, and redirects.
func (h *ClickHandler) HandleClick(c *gin.Context) {
	params, err := parseClickParams(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if !h.verifySignature(c, params) {
		return
	}

	generated := time.Unix(params.Timestamp, 0)
	if time.Since(generated) > h.maxAge {
		c.JSON(http.StatusGone, gin.H{"error": "click URL expired"})
		return
	}

	h.enqueueEvent(params, generated, c.Request.UserAgent())

	c.Redirect(http.StatusFound, params.DestinationURL)
}

// verifySignature checks the HMAC signature and responds with 403 if invalid.
func (h *ClickHandler) verifySignature(c *gin.Context, params clickurl.ClickParams) bool {
	msg := params.Message()
	if !h.signer.Verify(msg, c.Query("sig")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "invalid signature"})
		return false
	}
	return true
}

// enqueueEvent builds a ClickEvent and sends it to the buffer.
func (h *ClickHandler) enqueueEvent(params clickurl.ClickParams, generated time.Time, userAgent string) {
	event := domain.ClickEvent{
		QueryID:         params.QueryID,
		ResultID:        params.ResultID,
		Position:        params.Position,
		Page:            params.Page,
		DestinationHash: hashURL(params.DestinationURL),
		UserAgentHash:   hashUA(userAgent),
		GeneratedAt:     generated,
		ClickedAt:       time.Now(),
	}
	if !h.buffer.Send(event) {
		h.logger.Warn("Click event buffer full, dropping event",
			infralogger.String("query_id", params.QueryID),
		)
	}
}

func parseClickParams(c *gin.Context) (clickurl.ClickParams, error) {
	q := c.Query("q")
	r := c.Query("r")
	pStr := c.Query("p")
	pgStr := c.Query("pg")
	tStr := c.Query("t")
	u := c.Query("u")

	if q == "" || r == "" || pStr == "" || tStr == "" || u == "" {
		return clickurl.ClickParams{}, errMissingParams
	}

	p, err := strconv.Atoi(pStr)
	if err != nil {
		return clickurl.ClickParams{}, errMissingParams
	}

	pg := defaultPage
	if pgStr != "" {
		pg, _ = strconv.Atoi(pgStr)
		if pg < defaultPage {
			pg = defaultPage
		}
	}

	t, err := strconv.ParseInt(tStr, 10, 64)
	if err != nil {
		return clickurl.ClickParams{}, errMissingParams
	}

	return clickurl.ClickParams{
		QueryID:        q,
		ResultID:       r,
		Position:       p,
		Page:           pg,
		Timestamp:      t,
		DestinationURL: u,
	}, nil
}

func hashURL(rawURL string) string {
	h := sha256.Sum256([]byte(rawURL))
	return hex.EncodeToString(h[:])
}

func hashUA(ua string) string {
	if ua == "" {
		return ""
	}
	h := sha256.Sum256([]byte(ua))
	return hex.EncodeToString(h[:])[:uaHashLength]
}
