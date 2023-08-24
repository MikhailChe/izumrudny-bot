package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"mikhailche/botcomod/tracer"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
	"go.uber.org/zap"
)

const upsert_telegram_chat_query = `
DECLARE $id AS Int64;
DELCARE $type AS Utf8;
DECLARE $first_name AS Utf8;
DECLARE $last_name AS Utf8;
DECLARE $username AS Utf8;
DECLARE $title AS Utf8;
DECLARE $is_forum AS Bool;

UPSERT INTO telegram_chat
	(id, type, first_name, last_name, username, title, is_forum)
VALUES
	($id, $type, $first_name, $last_name, $username, $title, $is_forum)
;
`

func UpsertTelegramChat(ydb *ydb.Driver) func (ctx context.Context, chat telebot.Chat)error{
	defer tracer.Trace("UpsertTelegramChat")()
	return func (ctx context.Context, chat telebot.Chat)error{
		return ydb.Table().Do(ctx, func(ctx context.Context, s table.Session)error{
			defer tracer.Trace("UpsertTelegramChat::Do")()
			_, _, err := s.Execute(
				ctx,
				table.DefaultTxControl(),
				upsert_telegram_chat_query,
				table.NewQueryParameters(
					table.ValueParam("$id", types.Int64Value(chat.ID)),
					table.ValueParam("$type", types.Utf8Value(chat.Type)),
					table.ValueParam("$first_name", types.Utf8Value(chat.FirstName)),
					table.ValueParam("$last_name", types.Utf8Value(chat.LastName)),
					table.ValueParam("$username", types.Utf8Value(chat.Username)),
					table.ValueParam("$title", types.Utf8Value(chat.Title)),
					table.ValueParam("$is_forum", types.BoolValue(chat.IsForum)),
				),
			)
			if err != nil {
				return fmt.Errorf("UPSERT INTO telegram_chat: %w", err)
			}
			return nil
		})
	}
}


