package shares

import (
	"net/http"
	"net/url"
	"strings"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/qrsvg"
)

// Address is a link's public address with its QR code, which a phone held
// to the screen opens without typing.
type Address struct {
	URL string `json:"url"`
	QR  string `json:"qr"`
}

// origin is where visitors reach the server. Behind a reverse proxy only
// the browser knows it for sure, so the client may name it; without that
// it comes from the request as the proxy forwarded it.
func origin(r *http.Request) (string, error) {
	if o := r.URL.Query().Get("origin"); o != "" {
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
			strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" {
			return "", v1.Errorf(http.StatusBadRequest, "origin has to be a scheme and a host, such as https://files.example.org")
		}
		return u.Scheme + "://" + u.Host, nil
	}
	scheme := "http"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}
	host := r.Header.Get("X-Forwarded-Host")
	if host == "" {
		host = r.Host
	}
	return scheme + "://" + host, nil
}

func (s *Service) qr(w http.ResponseWriter, r *http.Request) error {
	sh, err := s.owned(r)
	if err != nil {
		return err
	}
	base, err := origin(r)
	if err != nil {
		return err
	}
	address := base + "/s/" + sh.Slug
	svg, err := qrsvg.Draw(address)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, Address{URL: address, QR: svg})
}
