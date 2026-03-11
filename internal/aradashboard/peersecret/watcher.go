package peersecret

import (
	"context"
	"log/slog"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// AuthSetter can update a Bearer token at runtime.
type AuthSetter interface {
	SetAuth(secret string)
}

// peersFile is a minimal representation of arascanner's peers.yaml.
type peersFile struct {
	PeerGroup struct {
		Secret string `yaml:"secret"`
	} `yaml:"peer_group"`
}

// Watcher polls arascanner's peers.yaml and updates the client auth token
// when the peer group secret changes.
type Watcher struct {
	path   string
	client AuthSetter

	current string
	cancel  context.CancelFunc
	done    chan struct{}
}

// New creates a Watcher. Call Start() to begin polling.
func New(secretFile string, client AuthSetter) *Watcher {
	return &Watcher{
		path:   secretFile,
		client: client,
	}
}

// Start begins background polling every 30 seconds.
func (w *Watcher) Start() {
	ctx, cancel := context.WithCancel(context.Background())
	w.cancel = cancel
	w.done = make(chan struct{})

	w.reload()

	go func() {
		defer close(w.done)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				w.reload()
			}
		}
	}()
	slog.Info("Peer secret watcher started", "file", w.path)
}

// Stop halts background polling and waits for the goroutine to exit.
func (w *Watcher) Stop() {
	if w.cancel != nil {
		w.cancel()
		<-w.done
	}
}

func (w *Watcher) reload() {
	data, err := os.ReadFile(w.path)
	if err != nil {
		slog.Debug("Peer secret watcher: cannot read file", "file", w.path, "error", err)
		return
	}

	var pf peersFile
	if err := yaml.Unmarshal(data, &pf); err != nil {
		slog.Warn("Peer secret watcher: cannot parse file", "file", w.path, "error", err)
		return
	}

	secret := pf.PeerGroup.Secret
	if secret == w.current {
		return
	}

	prev := w.current
	w.current = secret
	w.client.SetAuth(secret)

	if prev == "" {
		slog.Info("Peer secret watcher: loaded secret from file", "file", w.path)
	} else {
		slog.Info("Peer secret watcher: secret changed, updated client auth", "file", w.path)
	}
}
