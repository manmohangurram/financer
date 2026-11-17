package mail

import (
	"strings"
	"testing"
)

func TestParseEmailMultipartPlainPreferred(t *testing.T) {
	raw := strings.Join([]string{
		`From: "HDFC Bank" <alerts@hdfcbank.net>`,
		`To: me@example.com`,
		`Subject: Debit alert`,
		`MIME-Version: 1.0`,
		`Content-Type: multipart/alternative; boundary="b1"`,
		``,
		`--b1`,
		`Content-Type: text/plain; charset="utf-8"`,
		`Content-Transfer-Encoding: quoted-printable`,
		``,
		`Your A/c XX2467 has been debited with INR 378.64 on 17 NOV 2026.`,
		``,
		`On Tue, 17 Nov 2026 at 10:00, You <me@example.com> wrote:`,
		`> older mail that must be dropped`,
		`--b1`,
		`Content-Type: text/html; charset="utf-8"`,
		``,
		`<html><body><p>HTML version</p></body></html>`,
		`--b1--`,
	}, "\r\n")

	got := ParseEmail(raw)
	if got.FromName != "HDFC Bank" {
		t.Fatalf("fromName = %q", got.FromName)
	}
	if !strings.Contains(got.Body, "INR 378.64") {
		t.Fatalf("body = %q", got.Body)
	}
	if strings.Contains(got.Body, "older mail") {
		t.Fatalf("quoted history was not cut: %q", got.Body)
	}
	if strings.Contains(got.Body, "HTML version") {
		t.Fatalf("plain part should win: %q", got.Body)
	}
	if strings.Contains(got.Body, "Delivered-To") || strings.Contains(got.Body, "MIME-Version") {
		t.Fatalf("headers leaked into the body: %q", got.Body)
	}
}

func TestParseEmailHTMLOnly(t *testing.T) {
	raw := strings.Join([]string{
		`From: alerts@bank.com`,
		`Subject: Statement`,
		`MIME-Version: 1.0`,
		`Content-Type: text/html; charset="utf-8"`,
		``,
		`<html><head><style>.x{color:red}</style><title>Ignore me</title></head>`,
		`<body><p>Dear customer,</p><ul><li>One</li><li>Two</li></ul>`,
		`<script>bad()</script><p>Bye</p></body></html>`,
	}, "\r\n")

	got := ParseEmail(raw)
	for _, want := range []string{"Dear customer", "• One", "• Two", "Bye"} {
		if !strings.Contains(got.Body, want) {
			t.Errorf("body is missing %q: %q", want, got.Body)
		}
	}
	for _, unwanted := range []string{"<p>", "<li>", "bad()", "color:red", "Ignore me"} {
		if strings.Contains(got.Body, unwanted) {
			t.Errorf("body still contains %q: %q", unwanted, got.Body)
		}
	}
	if got.FromName != "" {
		t.Errorf("fromName = %q, want empty for a bare address", got.FromName)
	}
}

func TestParseEmailFallsBackWithoutMIME(t *testing.T) {
	raw := "Delivered-To: me@example.com\nReceived: by mx with SMTP\nSubject: hi\n\nActual body line\nsecond line"
	got := ParseEmail(raw)
	if got.Body != "Actual body line\nsecond line" {
		t.Fatalf("body = %q", got.Body)
	}
}

func TestParseEmailPlainTextAndEmpty(t *testing.T) {
	raw := "Content-Type: text/plain\n\nINR 10 debited"
	if got := ParseEmail(raw); !strings.Contains(got.Body, "INR 10 debited") {
		t.Fatalf("body = %q", got.Body)
	}
	if got := ParseEmail(""); got.Body != "(no readable content)" {
		t.Fatalf("empty body = %q", got.Body)
	}
}

func TestCleanBodyCollapsesBlankLines(t *testing.T) {
	got := cleanBody("a\n\n\n\n\nb   \n")
	if got != "a\n\nb" {
		t.Fatalf("cleanBody = %q", got)
	}
}
