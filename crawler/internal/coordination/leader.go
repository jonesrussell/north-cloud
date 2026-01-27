package coordination

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	infralogger "github.com/north-cloud/infrastructure/logger"
	"github.com/redis/go-redis/v9"
)

const (
	// DefaultLeaderTTL is the default leader election TTL.
	DefaultLeaderTTL = 30 * time.Second

	// DefaultLeaderRenewalInterval is the default interval for renewing leadership.
	DefaultLeaderRenewalInterval = 10 * time.Second

	// DefaultElectionRetryInterval is the default interval between election attempts.
	DefaultElectionRetryInterval = 5 * time.Second

	// renewalDivisor is used to calculate renewal interval from TTL.
	renewalDivisor = 3
)

var (
	// ErrNotLeader is returned when a leader-only operation is attempted by a non-leader.
	ErrNotLeader = errors.New("not the leader")

	// ErrElectionCancelled is returned when election is cancelled.
	ErrElectionCancelled = errors.New("election cancelled")
)

// LeaderElection provides Redis-based leader election.
type LeaderElection struct {
	client           *redis.Client
	key              string
	id               string
	ttl              time.Duration
	renewalInterval  time.Duration
	electionInterval time.Duration
	logger           infralogger.Logger

	isLeader atomic.Bool
	stopCh   chan struct{}
	wg       sync.WaitGroup

	// Callbacks
	onElected func()
	onLost    func()
}

// LeaderConfig holds configuration for leader election.
type LeaderConfig struct {
	Key              string        // Redis key for leadership
	TTL              time.Duration // Leadership TTL (default: 30s)
	RenewalInterval  time.Duration // Renewal interval (default: 10s)
	ElectionInterval time.Duration // Election retry interval (default: 5s)
	OnElected        func()        // Called when leadership is acquired
	OnLost           func()        // Called when leadership is lost
}

// DefaultLeaderConfig returns a LeaderConfig with sensible defaults.
func DefaultLeaderConfig(key string) LeaderConfig {
	return LeaderConfig{
		Key:              key,
		TTL:              DefaultLeaderTTL,
		RenewalInterval:  DefaultLeaderRenewalInterval,
		ElectionInterval: DefaultElectionRetryInterval,
	}
}

// NewLeaderElection creates a new leader election instance.
func NewLeaderElection(client *redis.Client, cfg LeaderConfig, logger infralogger.Logger) (*LeaderElection, error) {
	if cfg.Key == "" {
		return nil, errors.New("leader key is required")
	}

	if cfg.TTL <= 0 {
		cfg.TTL = DefaultLeaderTTL
	}
	if cfg.RenewalInterval <= 0 {
		cfg.RenewalInterval = DefaultLeaderRenewalInterval
	}
	if cfg.ElectionInterval <= 0 {
		cfg.ElectionInterval = DefaultElectionRetryInterval
	}

	// Ensure renewal happens before TTL expires
	if cfg.RenewalInterval >= cfg.TTL {
		cfg.RenewalInterval = cfg.TTL / renewalDivisor
	}

	return &LeaderElection{
		client:           client,
		key:              cfg.Key,
		id:               uuid.New().String(),
		ttl:              cfg.TTL,
		renewalInterval:  cfg.RenewalInterval,
		electionInterval: cfg.ElectionInterval,
		logger:           logger,
		stopCh:           make(chan struct{}),
		onElected:        cfg.OnElected,
		onLost:           cfg.OnLost,
	}, nil
}

// Start begins the leader election process.
func (l *LeaderElection) Start(ctx context.Context) {
	l.wg.Add(1)
	go l.run(ctx)
}

// Stop stops the leader election and releases leadership if held.
func (l *LeaderElection) Stop(ctx context.Context) error {
	close(l.stopCh)
	l.wg.Wait()

	// Release leadership if held
	if l.isLeader.Load() {
		return l.resign(ctx)
	}
	return nil
}

// IsLeader returns true if this instance is the leader.
func (l *LeaderElection) IsLeader() bool {
	return l.isLeader.Load()
}

// ID returns the unique identifier for this instance.
func (l *LeaderElection) ID() string {
	return l.id
}

// GetLeaderID returns the current leader's ID, or empty string if no leader.
func (l *LeaderElection) GetLeaderID(ctx context.Context) (string, error) {
	val, err := l.client.Get(ctx, l.key).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get leader: %w", err)
	}
	return val, nil
}

// run is the main election loop.
func (l *LeaderElection) run(ctx context.Context) {
	defer l.wg.Done()

	ticker := time.NewTicker(l.electionInterval)
	defer ticker.Stop()

	renewalTicker := time.NewTicker(l.renewalInterval)
	defer renewalTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			l.handleLostLeadership()
			return
		case <-l.stopCh:
			l.handleLostLeadership()
			return
		case <-ticker.C:
			if !l.isLeader.Load() {
				l.tryBecomeLeader(ctx)
			}
		case <-renewalTicker.C:
			if l.isLeader.Load() {
				l.renewLeadership(ctx)
			}
		}
	}
}

// tryBecomeLeader attempts to acquire leadership.
func (l *LeaderElection) tryBecomeLeader(ctx context.Context) {
	acquired, err := l.client.SetNX(ctx, l.key, l.id, l.ttl).Result()
	if err != nil {
		l.logger.Error("failed to acquire leadership",
			infralogger.String("error", err.Error()),
		)
		return
	}

	if acquired {
		l.logger.Info("acquired leadership",
			infralogger.String("leader_id", l.id),
		)
		l.isLeader.Store(true)
		if l.onElected != nil {
			l.onElected()
		}
	}
}

// renewLeadership renews the leadership TTL.
func (l *LeaderElection) renewLeadership(ctx context.Context) {
	// Use Lua script to atomically check and extend
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("pexpire", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)

	result, err := script.Run(ctx, l.client, []string{l.key}, l.id, l.ttl.Milliseconds()).Int()
	if err != nil {
		l.logger.Error("failed to renew leadership",
			infralogger.String("error", err.Error()),
		)
		l.handleLostLeadership()
		return
	}

	if result == 0 {
		l.logger.Warn("lost leadership - key not held")
		l.handleLostLeadership()
	}
}

// resign releases leadership.
func (l *LeaderElection) resign(ctx context.Context) error {
	// Use Lua script to atomically check and delete
	script := redis.NewScript(`
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`)

	_, err := script.Run(ctx, l.client, []string{l.key}, l.id).Int()
	if err != nil {
		return fmt.Errorf("failed to resign leadership: %w", err)
	}

	l.handleLostLeadership()
	l.logger.Info("resigned leadership",
		infralogger.String("leader_id", l.id),
	)
	return nil
}

// handleLostLeadership handles loss of leadership.
func (l *LeaderElection) handleLostLeadership() {
	if l.isLeader.CompareAndSwap(true, false) {
		l.logger.Info("lost leadership",
			infralogger.String("leader_id", l.id),
		)
		if l.onLost != nil {
			l.onLost()
		}
	}
}

// RunIfLeader runs a function only if this instance is the leader.
func (l *LeaderElection) RunIfLeader(ctx context.Context, fn func(ctx context.Context) error) error {
	if !l.isLeader.Load() {
		return ErrNotLeader
	}
	return fn(ctx)
}
