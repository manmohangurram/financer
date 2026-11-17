package mail

import (
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/emersion/go-message/mail"
	"golang.org/x/net/html"
)

// ParsedEmail is the display form of a stored message: the decoded sender name
// and a plain-text body with the transport headers and quoted history removed.
// The raw message is still kept separately because template extraction scans it.
type ParsedEmail struct {
	FromName string
	Subject  string
	Body     string
}

const (
	// maxBodyChars bounds the display body; bank alerts are far smaller, this is
	// only a guard against pathological mail.
	maxBodyChars = 200_000
	// maxPartBytes bounds one MIME part read.
	maxPartBytes = 1 << 20
)

// quoteMarkers cut a reply chain off the body. They are anchored to the start
// of a line and deliberately narrow so ordinary prose is not truncated.
var quoteMarkers = []*regexp.Regexp{
	regexp.MustCompile(`(?mi)^on .{10,}wrote:\s*$`),
	regexp.MustCompile(`(?mi)^-{2,}\s*original message\s*-{2,}`),
	regexp.MustCompile(`(?m)^_{8,}\s*$`),
	regexp.MustCompile(`(?mi)^from: .+\r?\nsent: .+\r?\nto: .+`),
}

var headerLine = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9-]*:`)

// ParseEmail decodes a raw RFC 822 message into display text. It prefers the
// text/plain part, converts text/html when that is all there is, and degrades
// to stripping the header block if the message cannot be parsed as MIME.
func ParseEmail(raw string) ParsedEmail {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	out := ParsedEmail{}

	mr, err := mail.CreateReader(strings.NewReader(raw))
	if err == nil {
		if addrs, aerr := mr.Header.AddressList("From"); aerr == nil && len(addrs) > 0 {
			out.FromName = strings.TrimSpace(addrs[0].Name)
		}
		if subject, serr := mr.Header.Subject(); serr == nil {
			out.Subject = strings.TrimSpace(subject)
		}

		var plain, htmlParts []string
		for {
			part, perr := mr.NextPart()
			if perr == io.EOF {
				break
			}
			if perr != nil {
				break
			}
			inline, ok := part.Header.(*mail.InlineHeader)
			if !ok {
				// Attachments are not display content.
				continue
			}
			contentType, _, cerr := inline.ContentType()
			if cerr != nil {
				continue
			}
			data, rerr := io.ReadAll(io.LimitReader(part.Body, maxPartBytes))
			if rerr != nil {
				continue
			}
			switch contentType {
			case "text/plain":
				plain = append(plain, string(data))
			case "text/html":
				htmlParts = append(htmlParts, string(data))
			}
		}

		switch {
		case strings.TrimSpace(strings.Join(plain, "\n")) != "":
			out.Body = cleanBody(strings.Join(plain, "\n"))
		case len(htmlParts) > 0:
			out.Body = cleanBody(htmlToText(strings.Join(htmlParts, "\n")))
		default:
			out.Body = cleanBody(stripHeaderBlock(raw))
		}
	} else {
		out.Body = cleanBody(stripHeaderBlock(raw))
	}

	if out.Body == "" {
		out.Body = "(no readable content)"
	}
	return out
}

// cleanBody normalises whitespace and drops the quoted reply history.
func cleanBody(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = cutQuoted(s)
	s = stripHTMLComments(s)

	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.TrimRight(strings.ReplaceAll(line, "\t", " "), " \u00a0")
		if strings.TrimSpace(line) == "" {
			blank++
			if blank > 1 {
				continue
			}
			out = append(out, "")
			continue
		}
		blank = 0
		out = append(out, line)
	}
	s = strings.TrimSpace(strings.Join(out, "\n"))
	return truncateRunes(s, maxBodyChars)
}

var htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)

func stripHTMLComments(s string) string {
	return htmlComment.ReplaceAllString(s, "")
}

func cutQuoted(s string) string {
	cut := len(s)
	for _, re := range quoteMarkers {
		if loc := re.FindStringIndex(s); loc != nil && loc[0] < cut {
			cut = loc[0]
		}
	}
	if cut < len(s) {
		s = s[:cut]
	}
	return s
}

// htmlToText walks an HTML document and keeps its block structure as newlines.
// It never returns markup, so the reader can render the body as plain text.
func htmlToText(input string) string {
	z := html.NewTokenizer(strings.NewReader(input))
	var b strings.Builder
	skipDepth := 0

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		switch tt {
		case html.StartTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "head", "title", "noscript", "template":
				skipDepth++
			case "br":
				b.WriteByte('\n')
			case "p", "div", "section", "article", "table", "blockquote", "pre",
				"h1", "h2", "h3", "h4", "h5", "h6", "header", "footer":
				b.WriteString("\n\n")
			case "tr":
				b.WriteByte('\n')
			case "td", "th":
				b.WriteString("  ")
			case "li":
				b.WriteString("\n• ")
			}
		case html.SelfClosingTagToken:
			name, _ := z.TagName()
			if string(name) == "br" {
				b.WriteByte('\n')
			}
		case html.EndTagToken:
			name, _ := z.TagName()
			switch string(name) {
			case "script", "style", "head", "title", "noscript", "template":
				if skipDepth > 0 {
					skipDepth--
				}
			case "p", "div", "section", "article", "table", "blockquote", "pre",
				"h1", "h2", "h3", "h4", "h5", "h6", "header", "footer":
				b.WriteString("\n\n")
			case "li":
				b.WriteByte('\n')
			}
		case html.TextToken:
			if skipDepth == 0 {
				b.Write(z.Text())
			}
		}
	}
	return b.String()
}

// stripHeaderBlock is the fallback when the message is not valid MIME: drop the
// leading run of header lines and keep the rest.
func stripHeaderBlock(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	lines := strings.Split(raw, "\n")
	if len(lines) == 0 {
		return ""
	}
	// Find the first blank line; only treat what precedes it as headers when at
	// least one line actually looks like `Name: value`.
	end := len(lines)
	headers := 0
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			end = i + 1
			break
		}
		if headerLine.MatchString(line) {
			headers++
		} else if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			// Not a header line: this is content, keep from here.
			end = i
			break
		}
	}
	if headers == 0 {
		return raw
	}
	return strings.Join(lines[end:], "\n")
}

func truncateRunes(s string, limit int) string {
	if limit <= 0 || len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut])
}
