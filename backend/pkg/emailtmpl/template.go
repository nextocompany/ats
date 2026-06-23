package emailtmpl

import (
	"bytes"
	"html"
	"html/template"
	"net/url"
	"regexp"
	"strings"
)

// Brand palette (hex translations of the oklch CI tokens — email clients do not
// support oklch). #0B47B8 is the canonical CP Axtra blue.
const (
	colorBrand   = "#0B47B8"
	colorDanger  = "#D64545"
	colorWarning = "#D99A2B"
	colorInk     = "#1E2433"
	colorMuted   = "#6B7280"
	colorPage    = "#FBFBFE"
	colorCard    = "#FFFFFF"
	colorHair    = "#E3E6EB"
)

// viewModel is the template input. Doc is embedded so its fields promote
// (.Title, .Greeting, .Paragraphs, .Details, .CTA). The extra fields are
// pre-validated/typed so html/template emits them without sanitizing away
// (RuleColor as template.CSS avoids the ZgotmplZ marker; CTAURL as template.URL
// after an http(s) scheme check).
type viewModel struct {
	Doc
	RuleColor template.CSS
	CTAURL    template.URL
	HasCTA    bool
	RawBody   template.HTML // set only by RenderPlain (replaces the structured body)
}

// docTmpl is the single branded shell. Parsed once at init; a parse error fails
// fast at startup rather than per-send.
var docTmpl = template.Must(template.New("email").Parse(emailHTML))

// Render returns the branded HTML for a Doc. A non-http(s) CTA URL drops the
// button. If template execution fails (a method panic, never expected), it falls
// back to the plain-shell render so a send never carries an empty HTML body.
func Render(d Doc) string {
	vm := viewModel{Doc: d, RuleColor: ruleColor(d.Accent)}
	if d.CTA != nil {
		if u, ok := safeURL(d.CTA.URL); ok {
			vm.CTAURL = u
			vm.HasCTA = true
		}
	}
	var buf bytes.Buffer
	if err := docTmpl.Execute(&buf, vm); err != nil {
		return RenderPlain(d.PlainText())
	}
	return buf.String()
}

// RenderPlain wraps an arbitrary plain-text body in the same branded shell. It is
// the universal fallback at the email bridge so even an email that supplies no Doc
// is still branded. The body is HTML-escaped and bare http(s) URLs are linkified.
func RenderPlain(body string) string {
	vm := viewModel{RuleColor: template.CSS(colorBrand), RawBody: plainBodyHTML(body)}
	var buf bytes.Buffer
	if err := docTmpl.Execute(&buf, vm); err != nil {
		return "<pre>" + html.EscapeString(body) + "</pre>"
	}
	return buf.String()
}

func ruleColor(accent string) template.CSS {
	switch accent {
	case AccentDanger:
		return template.CSS(colorDanger)
	case AccentWarning:
		return template.CSS(colorWarning)
	default:
		return template.CSS(colorBrand)
	}
}

// safeURL returns a trusted template.URL only for http/https; anything else
// (javascript:, data:, mailto:, empty) is rejected so the button is dropped.
func safeURL(raw string) (template.URL, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	return template.URL(raw), true
}

var urlRe = regexp.MustCompile(`https?://[^\s]+`)

// plainBodyHTML escapes a plain body and linkifies bare http(s) URLs, paragraph
// per line. Escaping happens per text segment (and on the URL itself for attribute
// safety), so it is injection-safe even though it emits template.HTML.
func plainBodyHTML(body string) template.HTML {
	var b strings.Builder
	for _, line := range strings.Split(body, "\n") {
		if strings.TrimSpace(line) == "" {
			b.WriteString(`<div style="height:8px;line-height:8px;">&nbsp;</div>`)
			continue
		}
		b.WriteString(`<p style="margin:0 0 12px 0;">`)
		b.WriteString(linkifyLine(line))
		b.WriteString(`</p>`)
	}
	return template.HTML(b.String())
}

func linkifyLine(line string) string {
	idxs := urlRe.FindAllStringIndex(line, -1)
	if idxs == nil {
		return html.EscapeString(line)
	}
	var b strings.Builder
	last := 0
	for _, ix := range idxs {
		b.WriteString(html.EscapeString(line[last:ix[0]]))
		escURL := html.EscapeString(line[ix[0]:ix[1]])
		b.WriteString(`<a href="`)
		b.WriteString(escURL)
		b.WriteString(`" style="color:` + colorBrand + `;">`)
		b.WriteString(escURL)
		b.WriteString(`</a>`)
		last = ix[1]
	}
	b.WriteString(html.EscapeString(line[last:]))
	return b.String()
}

// emailHTML is the table-based, inline-CSS shell. All colors are static literals
// except the heading rule (RuleColor, a trusted template.CSS) and the CTA href
// (CTAURL, a validated template.URL). The Google-Fonts @import is progressive
// enhancement only — Gmail/Outlook strip it and fall back to the system stack, so
// no layout depends on Noto Sans Thai loading.
const emailHTML = `<!DOCTYPE html>
<html lang="th">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="color-scheme" content="light">
<style>
@import url('https://fonts.googleapis.com/css2?family=Noto+Sans+Thai:wght@400;500;600;700&display=swap');
body{margin:0;padding:0;background:#FBFBFE;}
a{color:#0B47B8;}
</style>
</head>
<body style="margin:0;padding:0;background:#FBFBFE;-webkit-font-smoothing:antialiased;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#FBFBFE;">
<tr><td align="center" style="padding:24px 12px;">
<table role="presentation" width="600" cellpadding="0" cellspacing="0" border="0" style="width:600px;max-width:600px;background:#FFFFFF;border:1px solid #E3E6EB;border-radius:8px;font-family:'Noto Sans Thai',system-ui,-apple-system,'Segoe UI',Tahoma,sans-serif;">

  <tr><td style="padding:20px 28px;border-bottom:1px solid #E3E6EB;">
    <span style="font-weight:700;letter-spacing:0.04em;color:#1E2433;font-size:16px;">CP&nbsp;AXTRA</span>
    <span style="color:#E3E6EB;padding:0 8px;">|</span>
    <span style="color:#6B7280;font-size:13px;font-weight:500;">Careers</span>
  </td></tr>

  <tr><td style="padding:28px 28px 0 28px;">
    <div style="width:32px;height:3px;background:{{.RuleColor}};border-radius:2px;margin-bottom:16px;">&nbsp;</div>
    {{if .Title}}<h1 style="margin:0 0 16px 0;font-size:22px;line-height:1.35;color:#1E2433;font-weight:700;">{{.Title}}</h1>{{end}}
  </td></tr>

  {{if .RawBody}}
  <tr><td style="padding:0 28px;color:#1E2433;font-size:15px;line-height:1.7;">{{.RawBody}}</td></tr>
  {{else}}
  <tr><td style="padding:0 28px;color:#1E2433;font-size:15px;line-height:1.7;">
    {{if .Greeting}}<p style="margin:0 0 16px 0;">{{.Greeting}}</p>{{end}}
    {{range .Paragraphs}}<p style="margin:0 0 16px 0;">{{.}}</p>{{end}}
  </td></tr>
  {{if .Details}}
  <tr><td style="padding:4px 28px 0 28px;">
    <table role="presentation" width="100%" cellpadding="0" cellspacing="0" border="0" style="background:#FBFBFE;border:1px solid #E3E6EB;border-radius:8px;">
      {{range .Details}}<tr>
        <td style="padding:10px 16px;color:#6B7280;font-size:13px;vertical-align:top;border-bottom:1px solid #E3E6EB;">{{.Label}}</td>
        <td style="padding:10px 16px;color:#1E2433;font-size:14px;font-weight:600;text-align:right;vertical-align:top;border-bottom:1px solid #E3E6EB;">{{.Value}}</td>
      </tr>{{end}}
    </table>
  </td></tr>
  {{end}}
  {{if .HasCTA}}
  <tr><td style="padding:24px 28px 4px 28px;">
    <table role="presentation" cellpadding="0" cellspacing="0" border="0"><tr>
    <td style="background:#0B47B8;border-radius:8px;">
      <a href="{{.CTAURL}}" target="_blank" style="display:inline-block;padding:12px 26px;color:#FFFFFF;font-size:15px;font-weight:600;text-decoration:none;">{{.CTA.Label}} →</a>
    </td>
    </tr></table>
  </td></tr>
  {{end}}
  {{if .Outro}}
  <tr><td style="padding:18px 28px 0 28px;color:#6B7280;font-size:13px;line-height:1.6;">{{.Outro}}</td></tr>
  {{end}}
  {{end}}

  <tr><td style="padding:28px 28px 24px 28px;">
    <div style="border-top:1px solid #E3E6EB;padding-top:16px;color:#6B7280;font-size:12px;line-height:1.6;">
      CP Axtra Careers · อีเมลอัตโนมัติ โปรดอย่าตอบกลับอีเมลฉบับนี้ · <a href="https://www.cpaxtra.com" style="color:#6B7280;text-decoration:underline;">cpaxtra.com</a>
    </div>
  </td></tr>

</table>
</td></tr>
</table>
</body>
</html>`
