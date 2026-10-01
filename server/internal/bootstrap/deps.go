package bootstrap

import (
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/open-nerve/NerveWiki/server/internal/modules/identity"
	"github.com/open-nerve/NerveWiki/server/internal/modules/instance"
	"github.com/open-nerve/NerveWiki/server/internal/modules/notebook"
	"github.com/open-nerve/NerveWiki/server/internal/modules/workspace"
	"github.com/open-nerve/NerveWiki/server/internal/platform/clock"
	"github.com/open-nerve/NerveWiki/server/internal/platform/config"
	"github.com/open-nerve/NerveWiki/server/internal/platform/httpserver"
	"github.com/open-nerve/NerveWiki/server/internal/platform/jobs"
	"github.com/open-nerve/NerveWiki/server/internal/platform/postgres"
	"github.com/open-nerve/NerveWiki/server/internal/platform/ratelimit"
	"github.com/open-nerve/NerveWiki/server/internal/shared"
)

// The Deps of each module as serve builds them from the configuration: one
// function per module, so that newApp reads as the order of assembly.

// identityDeps are identity's: the sign-up policy, the signing keys, the
// module's buckets and the deactivation's registrants.
func identityDeps(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger, limiter *ratelimit.Limiter,
	keys *identity.SigningKeys, policy identity.SignupPolicy,
) identity.Deps {
	limits := cfg.RateLimit
	vetoers, subscribers := deactivationRegistrants(pool)
	return identity.Deps{
		Pool:                   pool,
		Tx:                     postgres.NewTxManager(pool, cfg.Database.CommitTimeout),
		Clock:                  clock.System{},
		Logger:                 logger,
		SignupPolicy:           policy,
		SigningKeys:            keys,
		AccessTokenTTL:         cfg.Auth.AccessTokenTTL,
		SessionTTL:             cfg.Auth.SessionTTL,
		RefreshDeadline:        cfg.Auth.RefreshDeadline,
		SessionCleanupInterval: cfg.Auth.SessionCleanupInterval,
		Password:               passwordHashing(cfg.Auth.Password),
		RateLimits: identity.RateLimits{
			Limiter:      limiter,
			LoginIP:      bucket(limiter, "login_ip", limits.LoginIP),
			LoginIPEmail: bucket(limiter, "login_ip_email", limits.LoginIPEmail),
			RegisterIP:   bucket(limiter, "register_ip", limits.RegisterIP),
			PasswordUser: bucket(limiter, "password_user", limits.PasswordUser),
		},
		DeactivationVetoers:     vetoers,
		DeactivationSubscribers: subscribers,
	}
}

// instanceDeps are instance's: what GET /instance reports of the
// configuration.
func instanceDeps(cfg config.Config) instance.Deps {
	return instance.Deps{SignupEnabled: cfg.Auth.SignupEnabled, WorkspaceCreationEnabled: cfg.Workspace.CreationEnabled}
}

// workspaceDeps are workspace's: the decisions of the access module;
// identity's share of the account row for the memberships it grants, and
// its directory for the member list and the invitations; the invitations'
// MAC key; the extension points' registrants.
func workspaceDeps(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger, authorizer shared.Authorizer,
	invitationKey []byte,
) workspace.Deps {
	ext := workspaceRegistrants(pool, nil)
	return workspace.Deps{
		Pool:                          pool,
		Tx:                            postgres.NewTxManager(pool, cfg.Database.CommitTimeout),
		Clock:                         clock.System{},
		Logger:                        logger,
		Authorizer:                    authorizer,
		Accounts:                      identity.NewAccounts(pool),
		Directory:                     directory{identity.NewDirectory(pool)},
		InvitationKey:                 invitationKey,
		CreationEnabled:               cfg.Workspace.CreationEnabled,
		MembershipEndVetoers:          ext.endVetoers,
		MembershipEndSubscribers:      ext.endSubscribers,
		DeletionSubscribers:           ext.deletionSubscribers,
		MembershipRestoreSubscribers:  ext.restoreSubscribers,
		MembershipAdditionSubscribers: ext.additionSubscribers,
		MemberRoleChangeSubscribers:   ext.roleChangeSubscribers,
	}
}

// notebookDeps are the notebook module's dependencies: the workspace
// module's ports, identity's directory for the member list, and the
// registrants of its extension points.
func notebookDeps(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger, authorizer shared.Authorizer) notebook.Deps {
	ext := notebookRegistrants()
	return notebook.Deps{
		Pool:                  pool,
		Tx:                    postgres.NewTxManager(pool, cfg.Database.CommitTimeout),
		Clock:                 clock.System{},
		Logger:                logger,
		Authorizer:            authorizer,
		Workspaces:            workspace.NewWorkspaces(pool),
		WorkspaceMembers:      workspace.NewMemberships(pool),
		Profiles:              notebookProfiles{identity.NewDirectory(pool)},
		DeletionSubscribers:   ext.deletionSubscribers,
		VisibilitySubscribers: ext.visibilitySubscribers,
		ActivitySources:       ext.activitySources,
	}
}

// purgeJob is the purge of the modules' soft-deleted rows, on
// jobs.purge_interval and jobs.purge_retention.
func purgeJob(cfg config.Config, pool *pgxpool.Pool, logger *slog.Logger) jobs.Job {
	return jobs.PurgeJob(purgers(pool), jobs.PurgeConfig{Interval: cfg.Jobs.PurgeInterval, Retention: cfg.Jobs.PurgeRetention, Logger: logger})
}

// apiConfig is the per-route middlewares' configuration: the modules'
// authenticator, public operations and shorter deadlines, and the
// platform's buckets.
func apiConfig(cfg config.Config, logger *slog.Logger, limiter *ratelimit.Limiter, authenticator httpserver.Authenticator,
	public []string, timeouts map[string]time.Duration,
) httpserver.APIConfig {
	limits := cfg.RateLimit
	return httpserver.APIConfig{
		Logger:           logger,
		Authenticator:    authenticator,
		PublicOperations: public,
		MaxBodyBytes:     cfg.Server.MaxBodyBytes,
		RequestTimeout:   cfg.Server.RequestTimeout,
		RequestTimeouts:  timeouts,
		TrustedProxies:   cfg.Server.TrustedProxies,
		IPv6PrefixLen:    limits.IPv6PrefixLen,
		Anonymous:        bucket(limiter, "anonymous", limits.Anonymous),
		Authenticated:    bucket(limiter, "authenticated", limits.Authenticated),
		AuthFailure:      bucket(limiter, "auth_failure", limits.AuthFailure),
	}
}

// passwordHashing is auth.password as identity takes it.
func passwordHashing(p config.PasswordConfig) identity.PasswordHashing {
	return identity.PasswordHashing{
		MemoryKiB:     p.Argon2MemoryKiB,
		Iterations:    p.Argon2Iterations,
		Parallelism:   p.Argon2Parallelism,
		MaxConcurrent: p.MaxConcurrentHashes,
		MaxWait:       p.MaxWait,
	}
}

// bucket is limiter's bucket of the configured rate, named after its key.
func bucket(limiter *ratelimit.Limiter, name string, c config.BucketConfig) *ratelimit.Bucket {
	return limiter.Bucket(name, ratelimit.Rate{PerMinute: c.PerMinute, Burst: c.Burst})
}
