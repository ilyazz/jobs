package certloader

import (
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// Loader is a cert loader. It
type Loader struct {
	// CertPath is where to load cert from
	CertPath string
	// KeyPath is where to load key from
	KeyPath string
	// Reload is cert reload interval
	Reload time.Duration

	lock sync.RWMutex
	cert *tls.Certificate

	started bool
	ticker  *time.Ticker
}

// Cert returns the last successfully loaded cert.
func (l *Loader) Cert() *tls.Certificate {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return l.cert
}

// Stop stops the cert watcher.
func (l *Loader) Stop() {
	l.lock.Lock()
	defer l.lock.Unlock()

	if !l.started {
		return
	}

	l.ticker.Stop()
	l.started = false
}

// Start loads the cert and starts the cert file watching ticker.
func (l *Loader) Start() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	if l.started {
		return fmt.Errorf("already started")
	}

	if l.Reload < 0 {
		l.Reload = 30 * time.Second
	}

	cert, err := l.load()
	if err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	l.cert = cert

	l.ticker = time.NewTicker(l.Reload)
	log.Info().Err(err).Msgf("reload cert from (%v/%v) every %v", l.CertPath, l.KeyPath, l.Reload)

	go func() {
		for range l.ticker.C {
			if err := l.reload(); err != nil {
				log.Warn().Err(err).Msg("failed to reload cert")
			} else {
				log.Info().Str("file", l.CertPath).Msg("reloaded cert")
			}
		}
	}()

	return nil
}

// reload loads the cert
func (l *Loader) load() (*tls.Certificate, error) {
	cert, err := tls.LoadX509KeyPair(l.CertPath, l.KeyPath)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// reload loads and replaces the cert
func (l *Loader) reload() error {
	l.lock.Lock()
	defer l.lock.Unlock()

	cert, err := l.load()
	if err != nil {
		return err
	}

	l.cert = cert
	return nil
}
