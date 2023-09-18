package repository

import (
	"context"
	"fmt"
	"mikhailche/botcomod/lib/tracer.v2"
	"path"

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

func (h *THouse) Scan(ctx context.Context, res result.Result) error {
	ctx, span := tracer.Open(ctx, tracer.Named("tHouse::Scan"))
	defer span.Close()
	return res.ScanNamed(
		named.OptionalWithDefault("id", &h.ID),
		named.OptionalWithDefault("number", &h.Number),
		named.OptionalWithDefault("construction", &h.Construction),
		named.OptionalWithDefault("rooms_min", &h.Rooms.Min),
		named.OptionalWithDefault("rooms_max", &h.Rooms.Max),
	)
}

type THouses []THouse

func (h *THouses) Scan(ctx context.Context, res result.Result) error {
	ctx, span := tracer.Open(ctx, tracer.Named("tHouses::Scan"))
	defer span.Close()
	var houses THouses
	for res.NextRow() {
		var house THouse
		if err := house.Scan(ctx, res); err != nil {
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

func NewHouseRepository(driver *ydb.Driver) *HouseRepository {
	return &HouseRepository{driver}
}

func (h *HouseRepository) Init(ctx context.Context) error {
	ctx, span := tracer.Open(ctx, tracer.Named("HouseRepository::Init"))
	defer span.Close()
	return h.DB.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
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
	ctx, span := tracer.Open(ctx, tracer.Named("HouseRepository::GetHouses"))
	defer span.Close()
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
		err = houses.Scan(ctx, res)
		return err
	}, table.WithIdempotent()); err != nil {
		return nil, err
	}
	return houses, nil
}
