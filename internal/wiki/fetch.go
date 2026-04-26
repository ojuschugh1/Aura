package wiki

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// FetchAndIngest downloads a URL, converts the HTML to plain text (stripping
// tags), and ingests it as a wiki source. This is the CLI equivalent of the
// gist's "Obsidian Web Clipper" — get web articles into the wiki without
// leaving the terminal.
func (e *Engine) FetchAndIngest(rawURL, title string) (*IngestResult, error) {
	// Fetch the URL.
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(rawURL)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", rawURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetch %s: HTTP %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024)) // 5MB limit
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	content := string(body)

	// Detect content type.
	contentType := resp.Header.Get("Content-Type")
	format := "text"

	if strings.Contains(contentType, "text/html") {
		// Convert HTML to readable text.
		content = htmlToText(content)
		format = "markdown"
	} else if strings.Contains(contentType, "application/json") {
		format = "json"
	} else if strings.Contains(contentType, "text/markdown") {
		format = "markdown"
	}

	// Auto-detect title from HTML <title> tag if not provided.
	if title == "" {
		title = extractHTMLTitle(string(body))
	}
	if title == "" {
		title = rawURL
	}

	// Prepend source URL to content.
	content = fmt.Sprintf("**Source URL:** %s\n\n%s", rawURL, content)

	return e.Ingest(title, content, format, rawURL)
}

// htmlToText strips HTML tags and converts to readable plain text.
// This is a simple heuristic converter — not a full HTML parser.
func htmlToText(html string) string {
	// Remove script and style blocks entirely.
	scriptRe := regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	html = scriptRe.ReplaceAllString(html, "")
	styleRe := regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	html = styleRe.ReplaceAllString(html, "")
	navRe := regexp.MustCompile(`(?is)<nav[^>]*>.*?</nav>`)
	html = navRe.ReplaceAllString(html, "")
	footerRe := regexp.MustCompile(`(?is)<footer[^>]*>.*?</footer>`)
	html = footerRe.ReplaceAllString(html, "")

	// Convert common block elements to markdown equivalents.
	h1Re := regexp.MustCompile(`(?i)<h1[^>]*>(.*?)</h1>`)
	html = h1Re.ReplaceAllString(html, "\n# $1\n")
	h2Re := regexp.MustCompile(`(?i)<h2[^>]*>(.*?)</h2>`)
	html = h2Re.ReplaceAllString(html, "\n## $1\n")
	h3Re := regexp.MustCompile(`(?i)<h3[^>]*>(.*?)</h3>`)
	html = h3Re.ReplaceAllString(html, "\n### $1\n")
	pRe := regexp.MustCompile(`(?i)<p[^>]*>(.*?)</p>`)
	html = pRe.ReplaceAllString(html, "\n$1\n")
	liRe := regexp.MustCompile(`(?i)<li[^>]*>(.*?)</li>`)
	html = liRe.ReplaceAllString(html, "\n- $1")
	brRe := regexp.MustCompile(`(?i)<br\s*/?>`)
	html = brRe.ReplaceAllString(html, "\n")

	// Convert inline elements.
	strongRe := regexp.MustCompile(`(?i)<(?:strong|b)[^>]*>(.*?)</(?:strong|b)>`)
	html = strongRe.ReplaceAllString(html, "**$1**")
	emRe := regexp.MustCompile(`(?i)<(?:em|i)[^>]*>(.*?)</(?:em|i)>`)
	html = emRe.ReplaceAllString(html, "*$1*")
	aRe := regexp.MustCompile(`(?i)<a[^>]*href="([^"]*)"[^>]*>(.*?)</a>`)
	html = aRe.ReplaceAllString(html, "[$2]($1)")
	codeRe := regexp.MustCompile(`(?i)<code[^>]*>(.*?)</code>`)
	html = codeRe.ReplaceAllString(html, "`$1`")

	// Strip all remaining HTML tags.
	tagRe := regexp.MustCompile(`<[^>]+>`)
	text := tagRe.ReplaceAllString(html, "")

	// Decode common HTML entities.
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")
	text = strings.ReplaceAll(text, "&nbsp;", " ")

	// Collapse excessive whitespace.
	multiNewline := regexp.MustCompile(`\n{3,}`)
	text = multiNewline.ReplaceAllString(text, "\n\n")
	text = strings.TrimSpace(text)

	return text
}

// extractHTMLTitle pulls the <title> content from HTML.
func extractHTMLTitle(html string) string {
	re := regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	m := re.FindStringSubmatch(html)
	if m == nil {
		return ""
	}
	title := strings.TrimSpace(m[1])
	// Strip HTML entities.
	title = strings.ReplaceAll(title, "&amp;", "&")
	title = strings.ReplaceAll(title, "&lt;", "<")
	title = strings.ReplaceAll(title, "&gt;", ">")
	return title
}
