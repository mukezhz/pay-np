package paynp

import (
	"html/template"
	"net/http"
	"net/url"
)

var autoSubmit = template.Must(template.New("checkout").Parse(`<!doctype html>
<html><head><meta charset="utf-8"><title>Redirecting to payment…</title></head>
<body onload="document.forms[0].submit()">
<form method="post" action="{{.URL}}">
{{range $k, $v := .Fields}}<input type="hidden" name="{{$k}}" value="{{$v}}">
{{end}}<noscript><button type="submit">Continue to payment</button></noscript>
</form></body></html>`))

// Write redirects (GET) or renders an auto-submitting form (POST).
func (c *Checkout) Write(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Cache-Control", "no-store")
	if c.Method != http.MethodPost {
		target, err := c.RedirectURL()
		if err != nil {
			return err
		}
		http.Redirect(w, r, target, http.StatusFound)
		return nil
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return autoSubmit.Execute(w, c)
}

func (c *Checkout) RedirectURL() (string, error) {
	u, err := url.Parse(c.URL)
	if err != nil {
		return "", err
	}
	if len(c.Fields) == 0 {
		return u.String(), nil
	}
	q := u.Query()
	for k, v := range c.Fields {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}
