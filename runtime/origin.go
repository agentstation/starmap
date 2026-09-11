package runtime

import (
	"time"

	"github.com/agentstation/starmap/pkg/catalogs/permission"
	"github.com/agentstation/starmap/pkg/catalogs/storage"
	"github.com/agentstation/starmap/pkg/errors"
)

// OriginConfig explicitly authorizes this runtime to publish and issue permissions for one catalog authority.
// The complete catalog selected by the runtime defines this policy's permitted catalog.
// Clock must supply qualified, cached time evidence. An unknown reading prevents receipt issuance.
// The deployment owns clock qualification and access control for all catalog mutations.
type OriginConfig struct {
	AuthorityID string
	PolicyID    string
	// Bootstrap permits the runtime to adopt an existing ordinary catalog store.
	// It cannot change an established authority or reset its sequence.
	Bootstrap          bool
	PermissionLifetime time.Duration
	Clock              func() permission.ClockReading
}

type authorityOrigin struct {
	config    OriginConfig
	publisher *permission.Publisher
	issuer    *permission.Issuer
	store     *originPublicationStore
}

// WithAuthorityOrigin selects the sole publication store and authorizes a catalog origin.
// Its store takes precedence over stores passed through WithClientOptions, independent of option order.
// Source, workspace, and acquisition options still apply. An authoritative subscriber cannot also be an origin.
// All writers to the underlying store must enforce the same authority publication contract.
func WithAuthorityOrigin(store storage.Store, config OriginConfig) Option {
	return func(o *options) error {
		if o.origin != nil {
			return originError("only one origin can be configured")
		}
		publisher, err := permission.NewPublisher(store, permission.PublisherConfig{AuthorityID: config.AuthorityID, PolicyID: config.PolicyID})
		if err != nil {
			return err
		}
		if _, ok := store.(storage.AuthorityHeadReader); !ok {
			return originError("store must support current authority observations")
		}
		issuer, err := permission.NewIssuer(publisher, permission.IssuerConfig{AuthorityID: config.AuthorityID, PolicyID: config.PolicyID, Lifetime: config.PermissionLifetime, Clock: config.Clock})
		if err != nil {
			return err
		}
		o.origin = &authorityOrigin{config: config, publisher: publisher, issuer: issuer, store: &originPublicationStore{Publisher: publisher, bootstrap: config.Bootstrap}}
		return nil
	}
}

func originError(message string) error {
	return &errors.ConfigError{Component: "catalog authority origin", Message: message}
}
