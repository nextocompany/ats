// Package emailtmpl renders CP Axtra-branded transactional emails. A Doc is the
// single source of an email's content: it renders to both a clean plain-text body
// (shared by LINE/Teams and the email text/plain part, so the channels never
// drift) and a branded HTML body. Pure rendering — no business copy lives here
// except the static frame chrome (wordmark, footer). Colors are hex (email clients
// do not support the oklch design tokens); the layout is table-based with inline
// CSS so it survives Gmail/Outlook.
package emailtmpl

import "strings"

// Accent tints the heading rule. The default ("") is CP Axtra blue.
const (
	AccentDefault = ""
	AccentDanger  = "danger"
	AccentWarning = "warning"
)

// DetailRow is one label/value line in the detail card (e.g. {"ตำแหน่ง", "แคชเชียร์"}).
// The Label may carry a leading emoji (📅 💻 📍) — it renders in both the plain body
// (so LINE keeps its emoji) and the HTML detail card.
type DetailRow struct {
	Label string
	Value string
}

// CTA is the primary call-to-action button. URL must be http(s); Render drops the
// button for any other scheme. PlainText always includes it as a "Label: URL" line
// so the link stays tappable in LINE.
type CTA struct {
	Label string
	URL   string
}

// Doc is the content of one email. Render(d) produces branded HTML; d.PlainText()
// produces the plain body shared by LINE/Teams and the email text/plain part.
type Doc struct {
	Title      string      // email subject + H1 heading
	Greeting   string      // e.g. "สวัสดีคุณสมชาย" — use Greeting(name)
	Paragraphs []string    // body paragraphs (natural sentences)
	Details    []DetailRow // optional structured rows (date/time, position, score…)
	CTA        *CTA        // optional primary action
	Outro      string      // optional muted closing note
	Accent     string      // AccentDefault | AccentDanger | AccentWarning
}

// Greeting builds the standard Thai greeting line, falling back to a neutral
// greeting when the name is empty (mirrors the prior per-builder logic).
func Greeting(fullName string) string {
	if strings.TrimSpace(fullName) == "" {
		return "สวัสดีค่ะ"
	}
	return "สวัสดีคุณ" + fullName
}

// PlainText renders the deterministic plain-text body. This is the no-drift
// contract between channels: LINE/Teams and the email text/plain part all use it.
func (d Doc) PlainText() string {
	var b strings.Builder
	write := func(s string) { b.WriteString(s) }
	sep := func() {
		if b.Len() > 0 {
			write("\n\n")
		}
	}

	if d.Greeting != "" {
		write(d.Greeting)
	}
	for _, p := range d.Paragraphs {
		if p == "" {
			continue
		}
		sep()
		write(p)
	}
	if len(d.Details) > 0 {
		sep()
		for i, r := range d.Details {
			if i > 0 {
				write("\n")
			}
			write(r.Label)
			write(": ")
			write(r.Value)
		}
	}
	if d.CTA != nil && d.CTA.URL != "" {
		sep()
		if d.CTA.Label != "" {
			write(d.CTA.Label)
			write(": ")
		}
		write(d.CTA.URL)
	}
	if d.Outro != "" {
		sep()
		write(d.Outro)
	}
	return b.String()
}
