package email

import (
	"fmt"
	"html"
	"time"
)

func codeEmailHTML(title, intro, code string, ttl time.Duration) string {
	minutes := int(ttl / time.Minute)
	if minutes < 1 {
		minutes = 1
	}

	return fmt.Sprintf(
		`<html><body><h2>%s</h2><p>%s</p><p style="font-size:24px;font-weight:700;letter-spacing:4px;">%s</p><p>Код действует %d минут.</p><p>Если вы не запрашивали письмо, просто проигнорируйте его.</p></body></html>`,
		html.EscapeString(title),
		html.EscapeString(intro),
		html.EscapeString(code),
		minutes,
	)
}
