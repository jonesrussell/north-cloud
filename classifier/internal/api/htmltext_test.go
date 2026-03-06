package api_test

import (
	"testing"

	"github.com/jonesrussell/north-cloud/classifier/internal/api"
	"github.com/stretchr/testify/assert"
)

func TestExtractTextFromHTML_BasicPage(t *testing.T) {
	html := `<html><head><title>Test</title></head>
		<body><h1>Hello World</h1><p>This is content.</p></body></html>`

	text := api.ExtractTextFromHTML(html)
	assert.Contains(t, text, "Hello World")
	assert.Contains(t, text, "This is content.")
	assert.NotContains(t, text, "<h1>")
	assert.NotContains(t, text, "<p>")
}

func TestExtractTextFromHTML_StripsScriptAndStyle(t *testing.T) {
	html := `<html><body>
		<script>var x = 1;</script>
		<style>.foo { color: red; }</style>
		<p>Visible text here.</p>
	</body></html>`

	text := api.ExtractTextFromHTML(html)
	assert.Contains(t, text, "Visible text here.")
	assert.NotContains(t, text, "var x")
	assert.NotContains(t, text, "color: red")
}

func TestExtractTextFromHTML_StripsNavHeaderFooter(t *testing.T) {
	html := `<html><body>
		<nav><a href="/">Home</a><a href="/about">About</a></nav>
		<header><h1>Site Header</h1></header>
		<article><p>Article body content here.</p></article>
		<footer><p>Copyright 2026</p></footer>
	</body></html>`

	text := api.ExtractTextFromHTML(html)
	assert.Contains(t, text, "Article body content here.")
	assert.NotContains(t, text, "Home")
	assert.NotContains(t, text, "Site Header")
	assert.NotContains(t, text, "Copyright 2026")
}

func TestExtractTextFromHTML_CollapsesWhitespace(t *testing.T) {
	html := `<html><body>
		<p>  Hello   world  </p>
		<p>  Another   paragraph  </p>
	</body></html>`

	text := api.ExtractTextFromHTML(html)
	assert.NotContains(t, text, "  ")
}

func TestExtractTextFromHTML_EmptyHTML(t *testing.T) {
	assert.Empty(t, api.ExtractTextFromHTML(""))
}

func TestCountWords(t *testing.T) {
	assert.Equal(t, 0, api.CountWords(""))
	assert.Equal(t, 1, api.CountWords("hello"))
	assert.Equal(t, 3, api.CountWords("hello world test"))
	assert.Equal(t, 3, api.CountWords("  hello   world   test  "))
}
