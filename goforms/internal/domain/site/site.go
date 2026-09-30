package site

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/net/idna"
)

var ErrInvalid = errors.New("invalid site")

// Site is an organization-owned website identity, distinct from the tenant.
type Site struct {
	ID             string    `gorm:"column:uuid;primaryKey;type:uuid"`
	OrganizationID string    `gorm:"column:organization_id;type:uuid;not null"`
	Name           string    `gorm:"column:name;not null"`
	Origin         string    `gorm:"column:origin;not null"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (Site) TableName() string { return "sites" }

func New(organizationID, name, origin string) (*Site, error) {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 100 {
		return nil, ErrInvalid
	}
	canonical, err := NormalizeOrigin(origin)
	if err != nil {
		return nil, err
	}
	return &Site{ID: uuid.NewString(), OrganizationID: organizationID, Name: name, Origin: canonical}, nil
}

// NormalizeOrigin accepts a site origin, never a URL with path, credentials or query.
func NormalizeOrigin(raw string) (string, error) {
	if raw == "" || strings.TrimSpace(raw) != raw || len(raw) > 2048 {
		return "", ErrInvalid
	}
	u, err := url.Parse(raw)
	if err != nil || u.Opaque != "" || u.User != nil || u.Host == "" || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.Fragment != "" || strings.HasSuffix(raw, "#") || strings.HasSuffix(raw, "?") {
		return "", ErrInvalid
	}
	host := strings.ToLower(u.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	} else {
		host, err = idna.Lookup.ToASCII(strings.TrimSuffix(host, "."))
		if err != nil || host == "" || strings.ContainsAny(host, " /\\") {
			return "", ErrInvalid
		}
	}
	port := u.Port()
	if port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return "", ErrInvalid
		}
		port = strconv.Itoa(value)
	}
	localHTTP := u.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1")
	if u.Scheme != "https" && !localHTTP {
		return "", ErrInvalid
	}
	if ip := net.ParseIP(host); ip == nil && host != "localhost" && !strings.Contains(host, ".") {
		return "", ErrInvalid
	}
	if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
		port = ""
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	if port != "" {
		host = net.JoinHostPort(strings.Trim(host, "[]"), port)
	}
	return u.Scheme + "://" + host, nil
}
