package repository

import (
	"context"
	"fmt"
	"mikhailche/botcomod/tracer"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

const upsert_cached_telegram_chat_to_sender_mapping_query = `
DECLARE $chat_id AS Int64;
DECLARE $user_id AS Int64;

UPSERT INTO telegram_chat_to_user
(chat_id, user_id)
VALUES
($chat_id, $user_id)
;
`

// CACHED MAPPING OF TELEGRAM CHAT TO SENDER
func UpsertTelegramChatToUserMapping(ydb *ydb.Driver) func(ctx context.Context, chat, user int64) error {
	return func(ctx context.Context, chat, user int64) error {
		defer tracer.Trace("UpsertTelegramChatToUserMapping")()
		return ydb.Table().Do(ctx, func(ctx context.Context, sess table.Session) error {
			defer tracer.Trace("UpsertTelegramChatToUserMapping::Do")
			_, _, err := sess.Execute(
				ctx,
				table.DefaultTxControl(),
				upsert_cached_telegram_chat_to_sender_mapping_query,
				table.NewQueryParameters(
					table.ValueParam("$user_id", types.Int64Value(user)),
					table.ValueParam("$chat_id", types.Int64Value(chat)),
				),
			)
			if err != nil {
				return fmt.Errorf("UPSERT INTO telegram_chat_to_user: %w", err)
			}
			return nil
		}, table.WithIdempotent())
	}
}

const select_cached_telegram_chat_to_sender_mapping_query = `
DECLARE $user_id AS Int64;

SELECT
chat_id
FROM telegram_chat_to_user
WHERE user_id = $user_id
;
`

func SelectTelegramChatsByUserID(ydb *ydb.Driver) func(context.Context, int64) ([]int64, error) {
	return func(ctx context.Context, user int64) ([]int64, error) {
		defer tracer.Trace("SelectTelegramChatsByUserID")()
		var ids []int64
		if err := ydb.Table().Do(ctx, func(ctx context.Context, sess table.Session) error {
			defer tracer.Trace("SelectTelegramChatsByUserID::Do")
			_, res, err := sess.Execute(
				ctx,
				table.DefaultTxControl(),
				select_cached_telegram_chat_to_sender_mapping_query,
				table.NewQueryParameters(
					table.ValueParam("$user_id", types.Int64Value(user)),
				),
			)
			if err != nil {
				return fmt.Errorf("SELECT FROM telegram_chat_to_user: %w", err)
			}
			if !res.NextResultSet(ctx, "chat_id") {
				return fmt.Errorf("ошибка запроса: не обнаружил результатов в ответе SelectTelegramChatsByUserID")
			}
			for res.NextRow() {
				var chatID int64
				if err := res.ScanNamed(named.OptionalWithDefault("chat_id", &chatID)); err != nil {
					return fmt.Errorf("SelectTelegramChatsByUserID: Scan: %w", err)
				}
				ids = append(ids, chatID)
			}
			return res.Err()
		}, table.WithIdempotent()); err != nil {
			return nil, err
		}
		return ids, nil
	}
}
