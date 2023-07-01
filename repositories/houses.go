package repositories

import (
	"context"
	"fmt"
	"path"

	. "mikhailche/botcomod/tracer"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/options"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

type tRoomRange struct {
	Min int
	Max int
}

type THouse struct {
	ID           uint64
	Number       string
	Construction string
	Rooms        tRoomRange
}

func (h *THouse) Scan(res result.Result) error {
	defer Trace("tHouse::Scan")()
	return res.ScanNamed(
		named.OptionalWithDefault("id", &h.ID),
		named.OptionalWithDefault("number", &h.Number),
		named.OptionalWithDefault("construction", &h.Construction),
		named.OptionalWithDefault("rooms_min", &h.Rooms.Min),
		named.OptionalWithDefault("rooms_max", &h.Rooms.Max),
	)
}

type THouses []THouse

func (h *THouses) Scan(res result.Result) error {
	defer Trace("tHouses::Scan")()
	var houses THouses
	for res.NextRow() {
		var house THouse
		if err := house.Scan(res); err != nil {
			return fmt.Errorf("чтение домов: %w", err)
		}
		houses = append(houses, house)
	}
	*h = houses
	return res.Err()
}

type HouseRepository struct {
	DB *ydb.Driver
}

func (h *HouseRepository) Init(ctx context.Context) error {
	defer Trace("HouseRepository::Init")()
	return h.DB.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		print("Создаём таблицу house")
		return s.CreateTable(ctx, path.Join(h.DB.Name(), "house"),
			options.WithColumn("id", types.TypeUint64),
			options.WithColumn("number", types.Optional(types.TypeString)),
			options.WithColumn("construction", types.Optional(types.TypeString)),
			options.WithColumn("rooms_min", types.Optional(types.TypeInt16)),
			options.WithColumn("rooms_max", types.Optional(types.TypeInt16)),
			options.WithPrimaryKeyColumn("id"),
		)
	})
}

func (h *HouseRepository) GetHouses(ctx context.Context) (THouses, error) {
	defer Trace("HouseRepository::GetHouses")()
	var houses THouses
	if err := h.DB.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		_, res, err := s.Execute(ctx, table.DefaultTxControl(), `SELECT * FROM house`, table.NewQueryParameters())
		if err != nil {
			return fmt.Errorf("чтение домов: %w", err)
		}
		defer res.Close()
		if !res.NextResultSet(ctx) {
			return fmt.Errorf("не нашел результатов при чтении домов, а должен был найти хотя бы один")
		}
		err = houses.Scan(res)
		return err
	}, table.WithIdempotent()); err != nil {
		return nil, err
	}
	return houses, nil
}
