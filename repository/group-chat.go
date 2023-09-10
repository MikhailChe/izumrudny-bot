package repository

import (
	"context"
	"fmt"
	"path"

	"mikhailche/botcomod/tracer"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/options"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
	"go.uber.org/zap"
)

type TGroupChats []TGroupChat

type TGroupChat struct {
	Group             string
	Name              string
	Link              string
	Order             int64
	TelegramChatID    int64
	TelegramChatTitle string
	TelegramChatType  string
	AntiObscene       bool
}

func (h *TGroupChats) Scan(res result.Result) error {
	defer tracer.Trace("tHouses::Scan")()
	var chats []TGroupChat
	for res.NextRow() {
		var chat TGroupChat
		if err := chat.Scan(res); err != nil {
			return fmt.Errorf("чтение домов: %w", err)
		}
		chats = append(chats, chat)
	}
	*h = chats
	return res.Err()
}

func (h *TGroupChat) Scan(res result.Result) error {
	defer tracer.Trace("tHouse::Scan")()
	return res.ScanNamed(
		named.OptionalWithDefault("group", &h.Group),
		named.OptionalWithDefault("name", &h.Name),
		named.OptionalWithDefault("link", &h.Link),
		named.OptionalWithDefault("order", &h.Order),
		named.OptionalWithDefault("telegram_chat_id", &h.TelegramChatID),
		named.OptionalWithDefault("telegram_chat_title", &h.TelegramChatTitle),
		named.OptionalWithDefault("telegram_chat_type", &h.TelegramChatType),
		named.OptionalWithDefault("anti_obscene", &h.AntiObscene),
	)
}

type ChatRepository struct {
	db  *ydb.Driver
	log *zap.Logger
}

func NewGroupChatRepository(driver *ydb.Driver, log *zap.Logger) *ChatRepository {
	repo := &ChatRepository{driver, log}
	repo.Init(context.Background())
	return repo
}

func (h *ChatRepository) Init(ctx context.Context) error {
	defer tracer.Trace("ChatRepository::Init")()
	return h.db.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		return s.CreateTable(ctx, path.Join(h.db.Name(), "groupChat"),
			options.WithColumn("group", types.TypeString),
			options.WithColumn("name", types.TypeString),
			options.WithColumn("link", types.Optional(types.TypeString)),
			options.WithColumn("order", types.Optional(types.TypeInt64)),
			options.WithColumn("telegram_chat_id", types.Optional(types.TypeInt64)),
			options.WithColumn("telegram_chat_title", types.Optional(types.TypeUTF8)),
			options.WithColumn("telegram_chat_type", types.Optional(types.TypeUTF8)),
			options.WithPrimaryKeyColumn("group", "name"),
		)
	})
}

func (h *ChatRepository) GetGroupChats(ctx context.Context) (TGroupChats, error) {
	defer tracer.Trace("ChatRepository::GetGroupChats")()
	var chats TGroupChats
	if err := h.db.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		_, res, err := s.Execute(ctx, table.DefaultTxControl(), `SELECT * FROM groupChat ORDER BY order`, table.NewQueryParameters())
		if err != nil {
			return fmt.Errorf("чтение чатов: %w", err)
		}
		defer res.Close()
		if !res.NextResultSet(ctx) {
			return fmt.Errorf("не нашел результатов при чтении чатов, а должен был найти хотя бы один")
		}
		err = chats.Scan(res)
		return err
	}, table.WithIdempotent()); err != nil {
		return nil, err
	}
	return chats, nil
}

func (h *ChatRepository) UpdateChatByTelegramId(
	ctx context.Context,
	telegramChatID int64,
	telegramChatTitle string,
	telegramChatType string,
) error {
	defer tracer.Trace("ChatRepository::UpdateChatByTelegramId")()
	return h.db.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		_, _, err := s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $telegram_chat_id AS Int64;
		DECLARE $telegram_chat_title AS Utf8;
		DECLARE $telegram_chat_type AS Utf8;
		UPDATE groupChat 
		SET 
			telegram_chat_title=$telegram_chat_title, 
			telegram_chat_type=$telegram_chat_type 
		WHERE telegram_chat_id = $telegram_chat_id;`,
			table.NewQueryParameters(
				table.ValueParam("$telegram_chat_id", types.Int64Value(telegramChatID)),
				table.ValueParam("$telegram_chat_title", types.UTF8Value(telegramChatTitle)),
				table.ValueParam("$telegram_chat_type", types.UTF8Value(telegramChatType)),
			))
		h.log.Info("Executed UpdateChatByTelegramId query", zap.Error(err),
			zap.Int64("telegram_chat_id", telegramChatID),
			zap.String("telegram_chat_title", telegramChatTitle),
			zap.String("telegram_chat_type", telegramChatType),
		)
		if err != nil {
			return fmt.Errorf("UPSERT groupChat: %w", err)
		}
		return nil
	})
}
