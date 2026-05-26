package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

type oauthSleeperExtraArgMatcher struct {
	resetAt time.Time
}

func (m oauthSleeperExtraArgMatcher) Match(v driver.Value) bool {
	var raw []byte
	switch value := v.(type) {
	case string:
		raw = []byte(value)
	case []byte:
		raw = value
	default:
		return false
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return false
	}
	if payload[service.OAuthSleeperExtraSleepKey] != true {
		return false
	}
	resetRaw, ok := payload[service.OAuthSleeperExtraResetAtKey].(string)
	if !ok {
		return false
	}
	resetAt, err := time.Parse(time.RFC3339Nano, resetRaw)
	if err != nil || !resetAt.Equal(m.resetAt) {
		return false
	}
	graceRaw, ok := payload[service.OAuthSleeperExtraStickyGraceKey].(string)
	if !ok {
		return false
	}
	_, err = time.Parse(time.RFC3339Nano, graceRaw)
	return err == nil
}

func newOAuthSleeperAccountRepoSQLMock(t *testing.T) (*accountRepository, sqlmock.Sqlmock) {
	t.Helper()

	db, mock := newSQLMock(t)
	drv := entsql.OpenDB(dialect.Postgres, db)
	client := dbent.NewClient(dbent.Driver(drv))
	t.Cleanup(func() { _ = client.Close() })

	return &accountRepository{client: client, sql: db}, mock
}

func TestOAuthSleeperRepositorySetRateLimitedIfLaterUpdatesAndEnqueuesOutbox(t *testing.T) {
	repo, mock := newOAuthSleeperAccountRepoSQLMock(t)
	ctx := context.Background()
	id := int64(42)
	resetAt := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	previous := resetAt.Add(-time.Hour)

	mock.ExpectBegin()
	mock.ExpectQuery("WITH target AS").
		WithArgs(resetAt, id, "{}").
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_limit_reset_at"}).AddRow(previous))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, id, nil, nil, schedulerOutboxDedupWindow.Seconds()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, gotPrevious, err := repo.SetRateLimitedIfLater(ctx, id, resetAt)
	require.NoError(t, err)
	require.True(t, updated)
	require.NotNil(t, gotPrevious)
	require.Equal(t, previous, *gotPrevious)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOAuthSleeperRepositoryCreateEventAfterRateLimitWritesExtraMarkers(t *testing.T) {
	repo, mock := newOAuthSleeperAccountRepoSQLMock(t)
	ctx := context.Background()
	resetAt := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)

	event := &service.OAuthSleeperEvent{
		AccountID:          7,
		AccountName:        "oauth-openai",
		Platform:           service.PlatformOpenAI,
		Window:             "codex_7d",
		UtilizationPercent: 99.5,
		ThresholdPercent:   95,
		ResetAt:            resetAt,
		CreatedAt:          resetAt.Add(-2 * time.Hour),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("WITH target AS").
		WithArgs(resetAt, event.AccountID, oauthSleeperExtraArgMatcher{resetAt: resetAt}).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_limit_reset_at"}).AddRow(nil)).
		WillDelayFor(0)
	mock.ExpectQuery("INSERT INTO oauth_sleeper_events").
		WithArgs(
			event.AccountID,
			event.AccountName,
			event.Platform,
			event.Window,
			event.UtilizationPercent,
			event.ThresholdPercent,
			event.ResetAt,
			nil,
			event.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(101), event.CreatedAt))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, event.AccountID, nil, nil, schedulerOutboxDedupWindow.Seconds()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := repo.CreateOAuthSleeperEventAfterRateLimit(ctx, event)
	require.NoError(t, err)
	require.True(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOAuthSleeperRepositorySetRateLimitedIfLaterRollsBackWhenNoUpdate(t *testing.T) {
	repo, mock := newOAuthSleeperAccountRepoSQLMock(t)
	ctx := context.Background()
	id := int64(42)
	resetAt := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)

	mock.ExpectBegin()
	mock.ExpectQuery("WITH target AS").
		WithArgs(resetAt, id, "{}").
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_limit_reset_at"}))
	mock.ExpectRollback()

	updated, previous, err := repo.SetRateLimitedIfLater(ctx, id, resetAt)
	require.NoError(t, err)
	require.False(t, updated)
	require.Nil(t, previous)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOAuthSleeperRepositoryCreateEventAfterRateLimitCommitsAtomicPath(t *testing.T) {
	repo, mock := newOAuthSleeperAccountRepoSQLMock(t)
	ctx := context.Background()
	resetAt := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)
	previous := resetAt.Add(-time.Hour)
	createdAt := resetAt.Add(-2 * time.Hour)

	event := &service.OAuthSleeperEvent{
		AccountID:          7,
		AccountName:        "oauth-openai",
		Platform:           service.PlatformOpenAI,
		Window:             "codex_7d",
		UtilizationPercent: 99.5,
		ThresholdPercent:   95,
		ResetAt:            resetAt,
		CreatedAt:          createdAt,
	}

	mock.ExpectBegin()
	mock.ExpectQuery("WITH target AS").
		WithArgs(resetAt, event.AccountID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_limit_reset_at"}).AddRow(previous))
	mock.ExpectQuery("INSERT INTO oauth_sleeper_events").
		WithArgs(
			event.AccountID,
			event.AccountName,
			event.Platform,
			event.Window,
			event.UtilizationPercent,
			event.ThresholdPercent,
			event.ResetAt,
			previous,
			event.CreatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).AddRow(int64(100), createdAt))
	mock.ExpectExec("INSERT INTO scheduler_outbox").
		WithArgs(service.SchedulerOutboxEventAccountChanged, event.AccountID, nil, nil, schedulerOutboxDedupWindow.Seconds()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	updated, err := repo.CreateOAuthSleeperEventAfterRateLimit(ctx, event)
	require.NoError(t, err)
	require.True(t, updated)
	require.Equal(t, int64(100), event.ID)
	require.NotNil(t, event.PreviousRateLimitResetAt)
	require.Equal(t, previous, *event.PreviousRateLimitResetAt)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOAuthSleeperRepositoryCreateEventAfterRateLimitRollsBackOnEventFailure(t *testing.T) {
	repo, mock := newOAuthSleeperAccountRepoSQLMock(t)
	ctx := context.Background()
	resetAt := time.Date(2026, 5, 24, 15, 0, 0, 0, time.UTC)

	event := &service.OAuthSleeperEvent{
		AccountID:          7,
		AccountName:        "oauth-openai",
		Platform:           service.PlatformOpenAI,
		Window:             "codex_5h",
		UtilizationPercent: 99,
		ThresholdPercent:   95,
		ResetAt:            resetAt,
		CreatedAt:          resetAt.Add(-time.Hour),
	}

	mock.ExpectBegin()
	mock.ExpectQuery("WITH target AS").
		WithArgs(resetAt, event.AccountID, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"previous_rate_limit_reset_at"}).AddRow(nil))
	mock.ExpectQuery("INSERT INTO oauth_sleeper_events").
		WillReturnError(sql.ErrConnDone)
	mock.ExpectRollback()

	updated, err := repo.CreateOAuthSleeperEventAfterRateLimit(ctx, event)
	require.Error(t, err)
	require.False(t, updated)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestOAuthSleeperRepositoryListEventsPaginatesNewestFirst(t *testing.T) {
	db, mock := newSQLMock(t)
	repo := &accountRepository{sql: db}

	createdLater := time.Date(2026, 5, 24, 16, 0, 0, 0, time.UTC)
	createdEarlier := createdLater.Add(-time.Hour)
	resetAt := createdLater.Add(2 * time.Hour)
	previous := resetAt.Add(-time.Hour)

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM oauth_sleeper_events").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(12)))
	mock.ExpectQuery(`(?s)FROM oauth_sleeper_events\s+ORDER BY created_at DESC, id DESC\s+LIMIT \$1 OFFSET \$2`).
		WithArgs(10, 10).
		WillReturnRows(sqlmock.NewRows([]string{
			"id",
			"account_id",
			"account_name",
			"platform",
			"usage_window",
			"utilization_percent",
			"threshold_percent",
			"reset_at",
			"previous_rate_limit_reset_at",
			"created_at",
		}).
			AddRow(int64(2), int64(20), "newer", service.PlatformOpenAI, "codex_7d", 99.0, 95.0, resetAt, previous, createdLater).
			AddRow(int64(1), int64(10), "older", service.PlatformAnthropic, "session_window", 98.0, 95.0, resetAt, nil, createdEarlier))

	events, page, err := repo.ListOAuthSleeperEvents(context.Background(), pagination.PaginationParams{Page: 2, PageSize: 10})
	require.NoError(t, err)
	require.Len(t, events, 2)
	require.Equal(t, int64(2), events[0].ID)
	require.Equal(t, "newer", events[0].AccountName)
	require.NotNil(t, events[0].PreviousRateLimitResetAt)
	require.Equal(t, int64(1), events[1].ID)
	require.Nil(t, events[1].PreviousRateLimitResetAt)
	require.Equal(t, int64(12), page.Total)
	require.Equal(t, 2, page.Page)
	require.Equal(t, 10, page.PageSize)
	require.Equal(t, 2, page.Pages)
	require.NoError(t, mock.ExpectationsWereMet())
}
