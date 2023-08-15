package repositories

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
)

type TGroupChats []TGroupChat

type TGroupChat struct {
	Group string
	Name  string
	Link  string
	Order int64
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
	)
}

type ChatRepository struct {
	db *ydb.Driver
}

func NewGroupChatRepository(driver *ydb.Driver) *ChatRepository {
	repo := &ChatRepository{driver}
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
