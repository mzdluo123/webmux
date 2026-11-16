package relaysetting

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"

	protocol "github.com/mzdluo123/webmux/protocol/relay"
	"github.com/rainchan/webmux/internal/config"
	"github.com/rainchan/webmux/internal/settingsstore"
	"github.com/rainchan/webmux/internal/statefile"
)

var (
	ErrUnavailable   = errors.New("relay settings storage unavailable")
	ErrInvalid       = errors.New("invalid relay settings")
	ErrEnableBlocked = errors.New("relay requires pairing, public risk confirmation and a recovery Token of at least 43 characters")
	ErrNotPersisted  = errors.New("relay is off but disabling was not persisted; repair storage before restarting")
)

type Settings struct {
	Config      config.RelayConfig
	Seed        string
	PendingSeed string
	Binding     *protocol.Binding
	OwnerLogin  string
	Confirmed   bool
}

type storedFile struct {
	V               int               `json:"v"`
	Enabled         *bool             `json:"enabled"`
	Confirmed       *bool             `json:"confirmed"`
	ControlEndpoint string            `json:"controlEndpoint"`
	ControlAudience string            `json:"controlAudience"`
	Seed            string            `json:"seed"`
	PendingSeed     string            `json:"pendingSeed"`
	Binding         *protocol.Binding `json:"binding"`
	OwnerLogin      string            `json:"ownerLogin"`
}

// storedFileV1 is read only: control pins no longer define the public authority.
// Keep its strict schema and validation so corrupt legacy state still fails closed.
type storedFileV1 struct {
	V               int                   `json:"v"`
	Enabled         *bool                 `json:"enabled"`
	Confirmed       *bool                 `json:"confirmed"`
	ControlEndpoint string                `json:"controlEndpoint"`
	ControlAudience string                `json:"controlAudience"`
	InitialTrust    protocol.ServiceTrust `json:"initialTrust"`
	Seed            string                `json:"seed"`
	PendingSeed     string                `json:"pendingSeed"`
	Binding         *protocol.Binding     `json:"binding"`
	OwnerLogin      string                `json:"ownerLogin"`
}

func (f *storedFile) UnmarshalJSON(data []byte) error {
	var version struct {
		V int `json:"v"`
	}
	if err := json.Unmarshal(data, &version); err != nil {
		return err
	}
	switch version.V {
	case 1:
		var legacy storedFileV1
		if err := statefile.Decode(data, &legacy); err != nil {
			return err
		}
		if legacy.ControlEndpoint != "" || legacy.ControlAudience != "" {
			if err := legacy.InitialTrust.Validate(); err != nil {
				return ErrInvalid
			}
		} else if legacy.InitialTrust.Algorithm != "" || len(legacy.InitialTrust.Pins) != 0 {
			return ErrInvalid
		}
		*f = storedFile{V: 2, Enabled: legacy.Enabled, Confirmed: legacy.Confirmed, ControlEndpoint: legacy.ControlEndpoint, ControlAudience: legacy.ControlAudience, Seed: legacy.Seed, PendingSeed: legacy.PendingSeed, Binding: legacy.Binding, OwnerLogin: legacy.OwnerLogin}
		return nil
	case 2:
		type currentFile storedFile
		var current currentFile
		if err := statefile.Decode(data, &current); err != nil {
			return err
		}
		*f = storedFile(current)
		return nil
	default:
		return ErrInvalid
	}
}

func (storedFile) UnavailableErr() error { return ErrUnavailable }

type Service = settingsstore.Store[Settings, storedFile]

var storeSpec = settingsstore.Spec[Settings, storedFile]{
	ID: statefile.Relay,
	Decode: func(f storedFile, defaults Settings) (Settings, error) {
		if f.V != 2 || f.Enabled == nil || f.Confirmed == nil {
			return Settings{}, ErrInvalid
		}
		c := config.RelayConfig{Enabled: *f.Enabled, Transport: "wss", ControlEndpoint: f.ControlEndpoint, ControlAudience: f.ControlAudience}
		// Older unpaired settings could omit the authority entirely. Adopt the
		// configured defaults only when there is no identity to redirect.
		if c.ControlEndpoint == "" && c.ControlAudience == "" && f.Seed == "" && f.PendingSeed == "" && f.Binding == nil {
			c.ControlEndpoint = defaults.Config.ControlEndpoint
			c.ControlAudience = defaults.Config.ControlAudience
		}
		return validateBootstrap(defaults, Settings{Config: c, Seed: f.Seed, PendingSeed: f.PendingSeed, Binding: f.Binding, OwnerLogin: f.OwnerLogin, Confirmed: *f.Confirmed})
	},
	Normalize: validateBootstrap,
	Encode: func(s Settings) storedFile {
		return storedFile{V: 2, Enabled: &s.Config.Enabled, Confirmed: &s.Confirmed, ControlEndpoint: s.Config.ControlEndpoint, ControlAudience: s.Config.ControlAudience, Seed: s.Seed, PendingSeed: s.PendingSeed, Binding: s.Binding, OwnerLogin: s.OwnerLogin}
	},
	FailClosed: func(s *Settings) { s.Config.Enabled = false },
	Closed:     "relay settings service is closed",
}

func validateBootstrap(defaults, next Settings) (Settings, error) {
	// Never send persisted or pending identity material to a different authority.
	a, b := defaults.Config, next.Config
	if a.ControlEndpoint != "" && (a.ControlEndpoint != b.ControlEndpoint || a.ControlAudience != b.ControlAudience) {
		return Settings{}, ErrInvalid
	}
	// Pool sizes are local transport policy, not persisted identity state.
	next.Config.InteractiveConnections = defaults.Config.InteractiveConnections
	next.Config.BulkConnections = defaults.Config.BulkConnections
	return validateSettings(next)
}

func validateSettings(s Settings) (Settings, error) {
	c, err := s.Config.Validated()
	if err != nil {
		return Settings{}, ErrInvalid
	}
	s.Config = c
	if c.ControlEndpoint == "" && (s.Seed != "" || s.PendingSeed != "" || s.Binding != nil) {
		return Settings{}, ErrInvalid
	}
	for _, seed := range []string{s.Seed, s.PendingSeed} {
		if seed != "" {
			if _, err := PrivateKey(seed); err != nil {
				return Settings{}, err
			}
		}
	}
	if s.Binding != nil {
		if s.Seed == "" || s.Binding.Validate() != nil {
			return Settings{}, ErrInvalid
		}
		binding := *s.Binding
		s.Binding = &binding
	}
	if s.Config.Enabled && (!s.Confirmed || s.Binding == nil) {
		return Settings{}, ErrEnableBlocked
	}
	return s, nil
}

func PrivateKey(seed string) (ed25519.PrivateKey, error) {
	var data [ed25519.SeedSize]byte
	if len(seed) != base64.RawURLEncoding.EncodedLen(len(data)) {
		return nil, ErrInvalid
	}
	n, err := base64.RawURLEncoding.Strict().Decode(data[:], []byte(seed))
	if err != nil || n != len(data) {
		return nil, ErrInvalid
	}
	return ed25519.NewKeyFromSeed(data[:]), nil
}

func Open(stateDir string, defaults config.RelayConfig) *Service {
	// TOML cannot grant public-shell risk consent or restore an absent key.
	defaults.Enabled = false
	return settingsstore.Open(&storeSpec, stateDir, Settings{Config: defaults})
}

func Inspect(stateDir string, defaults config.RelayConfig) (Settings, bool, error) {
	defaults.Enabled = false
	return settingsstore.Inspect(&storeSpec, stateDir, Settings{Config: defaults})
}
