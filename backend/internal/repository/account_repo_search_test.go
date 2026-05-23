package repository

import (
	"testing"

	dbaccount "github.com/Wei-Shaw/sub2api/ent/account"
	"github.com/stretchr/testify/require"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
)

func TestAccountSearchPredicateIncludesCredentialsEmail(t *testing.T) {
	selector := entsql.Dialect(dialect.Postgres).
		Select("*").
		From(entsql.Table(dbaccount.Table))

	accountSearchPredicate("GOLDAZANOLA")(selector)

	query, args := selector.Query()
	require.Contains(t, query, `"accounts"."name" ILIKE`)
	require.Contains(t, query, `COALESCE("accounts"."credentials"->>'email', '')`)
	require.Contains(t, query, `COALESCE("accounts"."credentials"->>'email', '') ILIKE $2`)
	require.NotContains(t, query, `?`, "postgres SQL must not retain raw ? placeholders")
	require.Equal(t, []any{"%goldazanola%", "%GOLDAZANOLA%"}, args)
}
