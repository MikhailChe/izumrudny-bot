package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"mikhailche/botcomod/lib/tracer.v2"
	"time"

	"mikhailche/botcomod/lib/errors"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result/named"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
	"go.uber.org/zap"
)

type Car struct {
	LicensePlate string `json:"plate"`
}

type Apartment struct {
	HouseNumber     string `json:"house"`
	ApartmentNumber string `json:"appartment"`
	NeedApprove     bool   `json:"need_approve"`
}

// UserApartments модель json данных из таблицы user колонки appartments
type UserApartments []Apartment

func (a *UserApartments) UnmarshalJSON(bb []byte) error {
	if len(bb) == 0 {
		return nil
	}
	var apartments []Apartment
	if err := json.Unmarshal(bb, &apartments); err != nil {
		return fmt.Errorf("apartments decoding of [%s]: %w", string(bb), err)
	}
	*a = apartments
	return nil
}

type Cars []Car

func (c *Cars) UnmarshalJSON(bb []byte) error {
	if len(bb) == 0 {
		return nil
	}
	var cars []Car
	if err := json.Unmarshal(bb, &cars); err != nil {
		return fmt.Errorf("cars decoding of [%s]: %w", string(bb), err)
	}
	*c = cars
	return nil
}

type tRegistrationEvents struct {
	Start *StartRegistrationEvent
}

type tRegistration struct {
	Events tRegistrationEvents
}

type User struct {
	ID                 int64
	Username           string
	Apartments         UserApartments
	Cars               Cars
	IsApprovedResident bool
	Registration       *tRegistration `json:"-"`
	Events             []any          `json:"-"`
}

func (u *User) Scan(ctx context.Context, res result.Result) error {
	ctx, span := tracer.Open(ctx, tracer.Named("User::Scan"))
	defer span.Close()
	return res.ScanNamed(
		named.Required("id", &u.ID),
		named.OptionalWithDefault("appartments", &u.Apartments),
		named.OptionalWithDefault("cars", &u.Cars),
		named.OptionalWithDefault("is_approved_resident", &u.IsApprovedResident),
		named.OptionalWithDefault("username", &u.Username),
	)
}

type UserEventRecord struct {
	User      int64
	Timestamp time.Time
	ID        string
	Type      string
	Event     UserEvent
}

func (u *UserEventRecord) Scan(ctx context.Context, res result.Result) error {
	ctx, span := tracer.Open(ctx, tracer.Named("UserEventRecord::Scan"))
	defer span.Close()
	var eventBytes []byte
	if err := res.ScanNamed(
		named.OptionalWithDefault("user", &u.User),
		named.OptionalWithDefault("timestamp", &u.Timestamp),
		named.OptionalWithDefault("id", &u.ID),
		named.OptionalWithDefault("type", &u.Type),
		named.OptionalWithDefault("event", &eventBytes),
	); err != nil {
		return fmt.Errorf("скан UserEventRecord: %w", err)
	}
	var event UserEvent = SelectType(ctx, u.Type)
	if event == nil {
		return fmt.Errorf("не удалось найти тип для %s", u.Type)
	}
	if err := json.Unmarshal(eventBytes, &event); err != nil {
		return fmt.Errorf("парсинг события %s [%s]: %w", u.Type, string(eventBytes), err)
	}
	u.Event = event
	return nil
}

type UserRepository struct {
	DB  *ydb.Driver
	log *zap.Logger
}

func NewUserRepository(ctx context.Context, ydb *ydb.Driver, log *zap.Logger) (*UserRepository, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("NewUserRepository"))
	defer span.Close()
	return &UserRepository{DB: ydb, log: log}, nil
}

// SELECT USER by USERNAME AND ID

type getUserOption = func(ctx context.Context, s table.Session) (txr table.Transaction, r result.Result, err error)

func (r *UserRepository) ByID(userID int64) func(ctx context.Context, s table.Session) (txr table.Transaction, r result.Result, err error) {
	return func(ctx context.Context, s table.Session) (txr table.Transaction, r result.Result, err error) {
		return s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $id AS Int64;
SELECT * FROM user WHERE id = $id LIMIT 1;`,
			table.NewQueryParameters(table.ValueParam("$id", types.Int64Value(userID))),
		)
	}
}

func (r *UserRepository) ByUsername(username string) func(ctx context.Context, s table.Session) (txr table.Transaction, r result.Result, err error) {
	return func(ctx context.Context, s table.Session) (txr table.Transaction, r result.Result, err error) {
		return s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $username AS Utf8;
SELECT * FROM user WHERE username = $username LIMIT 1;`,
			table.NewQueryParameters(table.ValueParam("$username", types.UTF8Value(username))),
		)
	}
}

func (r *UserRepository) ApplyEvents(ctx context.Context, user *User) error {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::ApplyEvents"))
	defer span.Close()
	return r.DB.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		_, res, err := s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $id AS Int64;
SELECT * FROM user_event WHERE user = $id ORDER BY user, timestamp, id;`,
			table.NewQueryParameters(table.ValueParam("$id", types.Int64Value(user.ID))),
		)
		if err != nil {
			return fmt.Errorf("SELECT user_event [id=%d]: %w", user.ID, err)
		}
		defer res.Close()
		if !res.NextResultSet(ctx) {
			return fmt.Errorf("не нашел result set для событий пользователя; невалидный запрос?")
		}
		for res.NextRow() {
			var event UserEventRecord
			if err := event.Scan(ctx, res); err != nil {
				return fmt.Errorf("не смог события пользователя: %w", err)
			}
			r.log.Info("Применяю собятие", zap.Any("event", event))
			event.Event.Apply(ctx, user)
			user.Events = append(user.Events, event)
		}
		return errors.ErrorfOrNil(res.Err(), "ApplyEvents [id=%d]", user.ID)
	})
}

func (r *UserRepository) GetUser(ctx context.Context, userQueryExecutor getUserOption) (*User, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::GetById"))
	defer span.Close()
	var user User
	if err := r.DB.Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::GetById::Do"))
		defer span.Close()
		_, res, err := userQueryExecutor(ctx, s)
		if err != nil {
			return fmt.Errorf("UserRepository::GetUser: %w", err)
		}
		_, span = tracer.Open(ctx, tracer.Named("UserRepository::GetById::DoUser"))
		defer span.Close()
		defer res.Close()
		if !res.NextResultSet(ctx) {
			return fmt.Errorf("не нашел result set для пользователя")
		}
		if !res.NextRow() {
			return fmt.Errorf("пользователь не найден")
		}
		if err := user.Scan(ctx, res); err != nil {
			return fmt.Errorf("скан пользователя %v: %w", res, err)
		}
		return r.ApplyEvents(ctx, &user)
	}, table.WithIdempotent()); err != nil {
		return nil, err
	}
	return &user, nil
}

var ErrNotFound = fmt.Errorf("not found")

func (r *UserRepository) FindByAppartment(ctx context.Context, house string, appartment string) (*User, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::FindByAppartment"))
	defer span.Close()
	var userIDs []int64
	if err := (*r.DB).Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		query := "SELECT `id` FROM `user`;"
		_, res, err := s.Execute(ctx, table.DefaultTxControl(), query, table.NewQueryParameters())
		if err != nil {
			return fmt.Errorf("select id from user: %w", err)
		}
		defer res.Close()
		for res.NextResultSet(ctx) {
			for res.NextRow() {
				var userID int64
				if err := res.ScanWithDefaults(&userID); err != nil {
					return fmt.Errorf("скан userID: %w", err)
				}
				userIDs = append(userIDs, userID)
			}
		}
		return res.Err()
	}, table.WithIdempotent()); err != nil {
		return nil, err
	}
	for _, userID := range userIDs {
		user, err := r.GetUser(ctx, r.ByID(userID))
		if err != nil {
			return nil, err
		}
		for _, appart := range user.Apartments {
			if appart.HouseNumber == house && appart.ApartmentNumber == appartment {
				return user, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (r *UserRepository) UpsertUsername(ctx context.Context, userID int64, username string) {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::UpsertUsername"))
	defer span.Close()
	if err := (*r.DB).Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		ctx, span := tracer.Open(ctx, tracer.Named("Do Upsert user"))
		defer span.Close()
		_, _, err := s.Execute(ctx,
			table.DefaultTxControl(),
			"DECLARE $id AS Int64; "+
				"DECLARE $username AS String; "+
				"UPSERT INTO `user` "+
				"(id, username)"+
				"VALUES ($id, $username);",
			table.NewQueryParameters(
				table.ValueParam("$id", types.Int64Value(userID)),
				table.ValueParam("$username", types.StringValueFromString(username)),
			),
		)
		if err != nil {
			return fmt.Errorf("UPSERT INTO `user`: %w", err)
		}
		return nil
	}, table.WithIdempotent()); err != nil {
		r.log.Error("Ошибка обновления пользователя", zap.Error(err))
	}
}

func (r *UserRepository) IsResident(ctx context.Context, userID int64) bool {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::IsResident"))
	defer span.Close()
	user, err := r.GetUser(ctx, r.ByID(userID))
	if err != nil {
		r.log.Error("Проблема определения резидентности", zap.Error(err))
		return false
	}
	return user.IsApprovedResident || user.Registration != nil
}

func (r *UserRepository) IsAdmin(ctx context.Context, userID int64) bool {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::IsAdmin"))
	defer span.Close()
	return userID == 257582730
}

func GenerateApproveCode(ctx context.Context, length int) string {
	ctx, span := tracer.Open(ctx, tracer.Named("GenerateApproveCode"))
	defer span.Close()
	var alphabet []rune = []rune("123456789ABCEHKMOPTX")
	var code []rune
	for i := 0; i < length; i++ {
		code = append(code, alphabet[rand.Intn(len(alphabet))])
	}
	return string(code)
}

func (r *UserRepository) StartRegistration(ctx context.Context, userID int64, updateID int64, houseNumber string, appartment string) (string, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::StartRegistration"))
	defer span.Close()
	const CodeLength = 5
	approveCode := GenerateApproveCode(ctx, CodeLength)
	var invalidCodes []string
	for i := 0; i < 5; i++ {
		invalidCodes = append(invalidCodes, GenerateApproveCode(ctx, CodeLength))
	}
	if err := r.LogEvent(ctx, userID, &StartRegistrationEvent{updateID, houseNumber, appartment, approveCode, invalidCodes}); err != nil {
		return "", fmt.Errorf("регистрация пользователя: %w", err)
	}
	return approveCode, nil
}

func (r *UserRepository) ConfirmRegistration(ctx context.Context, userID int64, event ConfirmRegistrationEvent) error {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::ConfirmRegistration"))
	defer span.Close()
	if err := r.LogEvent(ctx, userID, &event); err != nil {
		return fmt.Errorf("подтверждение регистрации: %w", err)
	}
	return nil
}

func (r *UserRepository) FailRegistration(ctx context.Context, userID int64, event FailRegistrationEvent) error {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::FailRegistration"))
	defer span.Close()
	if err := r.LogEvent(ctx, userID, &event); err != nil {
		return fmt.Errorf("проваленная регистрация: %w", err)
	}
	return nil
}

func (r *UserRepository) RegisterCarLicensePlate(ctx context.Context, userID int64, event RegisterCarLicensePlateEvent) error {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::RegisterCarLicensePlate"))
	defer span.Close()
	if err := r.LogEvent(ctx, userID, &event); err != nil {
		return fmt.Errorf("провалена регистрация авто: %w", err)
	}
	return nil
}

type UserRegistrationApproveToken struct {
	UserID      int64
	ApproveCode string
}
