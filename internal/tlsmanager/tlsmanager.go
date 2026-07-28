package tlsmanager

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"regexp"
	"strings"
	"sync"

	"github.com/caddyserver/certmagic"
	"github.com/statix/statix/internal/config"
)

type TLSEventKind string

const (
	TLSPending TLSEventKind = "tls_pending"
	TLSSuccess TLSEventKind = "tls_success"
	TLSError   TLSEventKind = "tls_error"
)

type TLSEvent struct {
	Kind    TLSEventKind `json:"kind"`
	Message string       `json:"message"`
}

var domainRegex = regexp.MustCompile(`^([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])(\.([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\-]{0,61}[a-zA-Z0-9]))+$`)

// ValidateDomain checks if a domain string conforms to RFC 1123.
func ValidateDomain(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("tlsmanager: domain must not be empty")
	}
	if len(domain) > 253 {
		return fmt.Errorf("tlsmanager: domain length exceeds 253 characters")
	}
	if !domainRegex.MatchString(domain) {
		return fmt.Errorf("tlsmanager: domain %q does not match RFC 1123 hostname format", domain)
	}
	return nil
}

// DNSResolver allows mocking net.Resolver in unit tests.
type DNSResolver interface {
	LookupHost(ctx context.Context, host string) ([]string, error)
}

type defaultResolver struct{}

func (d defaultResolver) LookupHost(ctx context.Context, host string) ([]string, error) {
	return net.DefaultResolver.LookupHost(ctx, host)
}

type Manager struct {
	cfg     *config.Config
	cfgPath string
	logger  *slog.Logger

	events chan TLSEvent
	Events <-chan TLSEvent

	mu           sync.Mutex
	magicConfig  *certmagic.Config
	activeDomain string
}

func New(cfg *config.Config, cfgPath string, logger *slog.Logger) *Manager {
	ch := make(chan TLSEvent, 10)
	return &Manager{
		cfg:     cfg,
		cfgPath: cfgPath,
		logger:  logger,
		events:  ch,
		Events:  ch,
	}
}

func (m *Manager) emitEvent(kind TLSEventKind, msg string) {
	evt := TLSEvent{Kind: kind, Message: msg}
	select {
	case m.events <- evt:
	default:
		m.logger.Warn("tlsmanager: event channel full, dropping event", "kind", kind, "msg", msg)
	}
}

func (m *Manager) CheckDNS(ctx context.Context, domain string, resolver DNSResolver) (bool, []string, error) {
	if resolver == nil {
		resolver = defaultResolver{}
	}

	addrs, err := resolver.LookupHost(ctx, domain)
	if err != nil {
		return false, nil, fmt.Errorf("tlsmanager: DNS lookup failed for %s: %w", domain, err)
	}

	return len(addrs) > 0, addrs, nil
}

func (m *Manager) Bind(ctx context.Context, domain string, resolver DNSResolver) error {
	domain = strings.TrimSpace(domain)
	if err := ValidateDomain(domain); err != nil {
		return err
	}

	if resolver == nil {
		resolver = defaultResolver{}
	}

	m.mu.Lock()
	m.activeDomain = domain
	m.mu.Unlock()

	// Async DNS check & certmagic binding
	go func() {
		m.emitEvent(TLSPending, fmt.Sprintf("Initiating domain binding for %s...", domain))

		match, ips, err := m.CheckDNS(ctx, domain, resolver)
		if err != nil {
			m.logger.Warn("tlsmanager: DNS pre-check error", "domain", domain, "error", err)
			m.emitEvent(TLSPending, fmt.Sprintf("DNS pre-check warning: %v", err))
		} else if !match || len(ips) == 0 {
			m.logger.Warn("tlsmanager: DNS pre-check did not return IP records", "domain", domain)
			m.emitEvent(TLSPending, "DNS pre-check warning: Domain does not resolve to an IP address.")
		} else {
			m.logger.Info("tlsmanager: DNS pre-check succeeded", "domain", domain, "ips", ips)
		}

		// Configure CertMagic
		certmagic.DefaultACME.CA = certmagic.LetsEncryptProductionCA
		magic := certmagic.NewDefault()
		magic.Storage = &certmagic.FileStorage{Path: "/etc/statix/certs"}

		m.mu.Lock()
		m.magicConfig = magic
		m.mu.Unlock()

		m.emitEvent(TLSPending, fmt.Sprintf("Obtaining TLS certificate for %s via Let's Encrypt...", domain))

		err = magic.ManageSync(ctx, []string{domain})
		if err != nil {
			m.logger.Error("tlsmanager: ACME certificate acquisition failed", "domain", domain, "error", err)
			m.emitEvent(TLSError, fmt.Errorf("TLS setup failed: %w", err).Error())
			return
		}

		// Update config
		m.cfg.Domain = domain
		m.cfg.TLSEnabled = true
		if saveErr := config.Save(m.cfgPath, m.cfg); saveErr != nil {
			m.logger.Error("tlsmanager: failed to save config after TLS bind", "error", saveErr)
		}

		m.logger.Info("tlsmanager: TLS binding successful", "domain", domain)
		m.emitEvent(TLSSuccess, fmt.Sprintf("HTTPS active for %s", domain))
	}()

	return nil
}

func (m *Manager) MagicConfig() *certmagic.Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.magicConfig
}
