package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"mikhailche/botcomod/handlers/middleware/ydbctx"
	"mikhailche/botcomod/lib/devbotsender"
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
	HouseID         uint64 `json:"house_id"`
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

type tPrivatePropertyItem struct {
	HouseID         uint64
	ApartmentNumber string
	Approved        bool
}

func (ppi tPrivatePropertyItem) Key() string {
	return fmt.Sprintf("%d:%s", ppi.HouseID, ppi.ApartmentNumber)
}

type tPrivatePropertySet struct {
	Items map[string]tPrivatePropertyItem
}

func (p *tPrivatePropertySet) Add(HouseID uint64, ApartmentNumber string) {
	if p.Items == nil {
		p.Items = make(map[string]tPrivatePropertyItem)
	}
	ppi := tPrivatePropertyItem{HouseID: HouseID, ApartmentNumber: ApartmentNumber}
	p.Items[ppi.Key()] = ppi
}

func (p *tPrivatePropertySet) Approve(id uint64, apartment string) {
	if p.Items == nil {
		p.Items = make(map[string]tPrivatePropertyItem)
	}
	ppi, ok := p.Items[tPrivatePropertyItem{HouseID: id, ApartmentNumber: apartment}.Key()]
	if !ok {
		return
	}
	ppi.Approved = true
	p.Items[ppi.Key()] = ppi
}

func (p *tPrivatePropertySet) RemoveIfNotApproved(id uint64, apartment string) {
	if p.Items == nil {
		p.Items = make(map[string]tPrivatePropertyItem)
	}
	ppi := p.Items[tPrivatePropertyItem{HouseID: id, ApartmentNumber: apartment}.Key()]
	if ppi.Approved {
		return
	}
	delete(p.Items, ppi.Key())
}

type User struct {
	ID                 int64
	Username           string
	Apartments         UserApartments
	Cars               Cars
	IsApprovedResident bool
	Registration       *tRegistration `json:"-"`
	PrivateProperty    tPrivatePropertySet
	Events             []any `json:"-"`
}

func (u *User) Scan(ctx context.Context, res result.Result) error {
	ctx, span := tracer.Open(ctx, tracer.Named("User::Scan"))
	defer span.Close()
	u.PrivateProperty.Items = make(map[string]tPrivatePropertyItem)
	return res.ScanNamed(
		named.Required("id", &u.ID),
		named.OptionalWithDefault("appartments", &u.Apartments),
		named.OptionalWithDefault("cars", &u.Cars),
		named.OptionalWithDefault("is_approved_resident", &u.IsApprovedResident),
		named.OptionalWithDefault("username", &u.Username),
	)
}

func (u *User) HavePendingRegistration() bool {
	for _, v := range u.PrivateProperty.Items {
		if v.Approved == false {
			return true
		}
	}
	return false
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
	var event = SelectType(ctx, u.Type)
	if event == nil {
		return fmt.Errorf("не удалось найти тип для %s", u.Type)
	}
	if err := json.Unmarshal(eventBytes, &event); err != nil {
		return fmt.Errorf("парсинг события %s [%s]: %w", u.Type, string(eventBytes), err)
	}
	u.Event = event
	return nil
}

type ydbDriver interface {
	Table() table.Client
}

type UserRepository struct {
	DB  ydbDriver
	log *zap.Logger
}

func NewUserRepository(ctx context.Context, ydb *ydb.Driver, log *zap.Logger) (*UserRepository, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("NewUserRepository"))
	defer span.Close()
	return &UserRepository{DB: ydb, log: log}, nil
}

// SELECT USER by USERNAME AND ID

type getUserOption = func(ctx context.Context, s table.Session) (*User, error)

func (r *UserRepository) postGetUserOptionToUserScanner(ctx context.Context, s table.Session, res result.Result, err error) (*User, error) {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	defer r.log.Debug("Закончил доставать пользователя из базы")
	if err != nil {
		return nil, fmt.Errorf("UserRepository::postGetUserOptionToUserScanner: %w", err)
	}
	defer res.Close()
	if !res.NextResultSet(ctx) {
		return nil, fmt.Errorf("не нашел result set для пользователя")
	}
	if !res.NextRow() {
		return nil, fmt.Errorf("postGetUserOptionToUserScanner: пользователь не найден: %w", res.Err())
	}
	var user User
	if err := user.Scan(ctx, res); err != nil {
		return nil, fmt.Errorf("скан пользователя %v: %w", res, err)
	}
	if err := r.applyEvents(ctx, s, &user); err != nil {
		return nil, fmt.Errorf("применение событий: %w", err)
	}
	return &user, nil
}

func (r *UserRepository) ByID(userID int64) func(ctx context.Context, s table.Session) (*User, error) {
	return func(ctx context.Context, s table.Session) (*User, error) {
		ctx, span := tracer.Open(ctx)
		defer span.Close()
		if user := CurrentUserFromContext(ctx); user != nil && user.ID == userID {
			return user, nil
		}
		_, res, err := s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $id AS Int64;
SELECT * FROM user WHERE id = $id LIMIT 1;`,
			table.NewQueryParameters(table.ValueParam("$id", types.Int64Value(userID))),
		)
		return r.postGetUserOptionToUserScanner(ctx, s, res, err)
	}
}

func (r *UserRepository) ByUsername(username string) func(ctx context.Context, s table.Session) (*User, error) {
	return func(ctx context.Context, s table.Session) (*User, error) {
		ctx, span := tracer.Open(ctx)
		defer span.Close()
		if user := CurrentUserFromContext(ctx); user != nil && user.Username == username {
			return user, nil
		}
		_, res, err := s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $username AS Utf8;
SELECT * FROM user WHERE username = $username LIMIT 1;`,
			table.NewQueryParameters(table.ValueParam("$username", types.UTF8Value(username))),
		)
		return r.postGetUserOptionToUserScanner(ctx, s, res, err)
	}
}

func (r *UserRepository) applyEvents(ctx context.Context, s table.Session, user *User) error {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::applyEvents"))
	defer span.Close()
	defer r.log.Debug("Закончил применять события")
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
		return fmt.Errorf("не нашел result set для событий пользователя; возможно невалидный запрос")
	}
	for res.NextRow() {
		var event UserEventRecord
		if err := event.Scan(ctx, res); err != nil {
			return fmt.Errorf("не смог события пользователя: %w", err)
		}
		r.log.Debug("Применяю собятие", zap.Any("event", event))
		event.Event.Apply(ctx, user)
		user.Events = append(user.Events, event)
	}
	return errors.ErrorfOrNil(res.Err(), "applyEvents [id=%d]", user.ID)
}

func (r *UserRepository) ClearEvents(ctx context.Context, userID int64) error {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	return r.smartExecute(ctx, func(ctx context.Context, s table.Session) error {
		ctx, span := tracer.Open(ctx)
		defer span.Close()
		_, _, err := s.Execute(
			ctx,
			table.DefaultTxControl(),
			"DECLARE $id AS Int64; DELETE FROM user_event WHERE user = $id;",
			table.NewQueryParameters(table.ValueParam("$id", types.Int64Value(userID))),
		)
		return err
	})
}

type currentUserInContextKeyType int

var currentUserInContextKey currentUserInContextKeyType

func CurrentUserFromContext(ctx context.Context) *User {
	u, _ := ctx.Value(currentUserInContextKey).(*User)
	return u
}

func PutCurrentUserToContext(ctx context.Context, user *User) context.Context {
	return context.WithValue(ctx, currentUserInContextKey, user)
}

func (r *UserRepository) GetUser(ctx context.Context, userQueryExecutor getUserOption) (*User, error) {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	defer r.log.Debug("Закончил UserRepository::GetUser")
	var user *User
	if err := r.smartExecute(ctx, func(ctx context.Context, s table.Session) error {
		ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::getUser::Do"))
		defer span.Close()
		var err error
		user, err = userQueryExecutor(ctx, s)
		if err != nil {
			return fmt.Errorf("userRepository::getUser::do: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepository) smartExecute(ctx context.Context, fn func(ctx context.Context, s table.Session) error) error {
	if sess := ydbctx.YdbSessionFromContext(ctx); sess != nil {
		return fn(ctx, sess)
	}
	return r.DB.Table().Do(ctx, fn, table.WithIdempotent())
}

func (r *UserRepository) GetAllUsers(ctx context.Context) ([]*User, error) {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	var users []*User
	var events []*UserEventRecord
	executeSelectAllUsers := func(ctx context.Context, s table.Session) error {
		ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::GetAllUsers::executeSelectAllUsers"))
		defer span.Close()
		var err error
		query := `SELECT * FROM user ORDER BY id;
		SELECT * FROM user_event ORDER BY user, timestamp, id;`
		_, res, err := s.Execute(ctx, table.DefaultTxControl(), query, table.NewQueryParameters())
		if err != nil {
			return fmt.Errorf("SELECT * FROM user: %w", err)
		}
		defer res.Close()
		res.NextResultSet(ctx)
		for res.NextRow() {
			var user = new(User)
			if err := user.Scan(ctx, res); err != nil {
				return fmt.Errorf("скан user: %w", err)
			}
			users = append(users, user)
		}
		res.NextResultSet(ctx)
		for res.NextRow() {
			var userEvent = new(UserEventRecord)
			if err := userEvent.Scan(ctx, res); err != nil {
				return fmt.Errorf("скан userevent: %w", err)
			}
			events = append(events, userEvent)
		}
		return res.Err()
	}
	if err := r.smartExecute(ctx, executeSelectAllUsers); err != nil {
		return nil, err
	}

	var i, j int
	for i < len(users) && j < len(events) {
		u := users[i]
		e := events[j]
		if e.User < u.ID {
			// TODO: WTF? events for deleted user?
			j++
			continue
		}
		if e.User > u.ID {
			//
			i++
			continue
		}
		if e.User != u.ID {
			return nil, fmt.Errorf("something wrong with this logic")
		}
		e.Event.Apply(ctx, u)
		j++
	}

	return users, nil
}

var ErrNotFound = fmt.Errorf("not found")

// FindByVehicleLicensePlate implements bot.UserByVehicleLicensePlateRepository.
func (r *UserRepository) FindByVehicleLicensePlate(ctx context.Context, vehicleLicensePlate string) (*User, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::FindByVehicleLicensePlate"))
	defer span.Close()
	users, err := r.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
		for _, car := range user.Cars {
			if car.LicensePlate == vehicleLicensePlate {
				return user, nil
			}
		}
	}
	return nil, ErrNotFound
}

func (r *UserRepository) FindByAppartment(ctx context.Context, house string, appartment string) (*User, error) {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::FindByAppartment"))
	defer span.Close()
	users, err := r.GetAllUsers(ctx)
	if err != nil {
		return nil, err
	}
	for _, user := range users {
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
	if err := r.smartExecute(ctx, func(ctx context.Context, s table.Session) error {
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
	}); err != nil {
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
	return user.IsApprovedResident
}

func (r *UserRepository) IsAdmin(ctx context.Context, userID int64) bool {
	_, span := tracer.Open(ctx, tracer.Named("UserRepository::IsAdmin"))
	defer span.Close()
	return userID == devbotsender.DeveloperID
}

func GenerateApproveCode(ctx context.Context, length int) string {
	_, span := tracer.Open(ctx, tracer.Named("GenerateApproveCode"))
	defer span.Close()
	alphabet := []rune("123456789ABCEHKMOPTX")
	var code []rune
	for i := 0; i < length; i++ {
		code = append(code, alphabet[rand.Intn(len(alphabet))])
	}
	return string(code)
}

func (r *UserRepository) StartRegistration(ctx context.Context, userID int64, updateID int64, houseID uint64, houseNumber string, apartment string) (string, error) {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	const CodeLength = 5
	approveCode := GenerateApproveCode(ctx, CodeLength)
	var invalidCodes []string
	for i := 0; i < 5; i++ {
		invalidCodes = append(invalidCodes, GenerateApproveCode(ctx, CodeLength))
	}
	if err := r.LogEvent(ctx, userID, &StartRegistrationEvent{
		UpdateID:     updateID,
		HouseID:      houseID,
		HouseNumber:  houseNumber,
		Apartment:    apartment,
		ApproveCode:  approveCode,
		InvalidCodes: invalidCodes,
	}); err != nil {
		return "", fmt.Errorf("регистрация пользователя: %w", err)
	}
	return approveCode, nil
}

func (r *UserRepository) ConfirmRegistration(ctx context.Context, userID int64, event ConfirmRegistrationEvent) error {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	if err := r.LogEvent(ctx, userID, &event); err != nil {
		return fmt.Errorf("подтверждение регистрации: %w", err)
	}
	return nil
}

func (r *UserRepository) FailRegistration(ctx context.Context, userID int64, event FailRegistrationEvent) error {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	if err := r.LogEvent(ctx, userID, &event); err != nil {
		return fmt.Errorf("проваленная регистрация: %w", err)
	}
	return nil
}

func (r *UserRepository) RegisterCarLicensePlate(ctx context.Context, userID int64, event RegisterCarLicensePlateEvent) error {
	ctx, span := tracer.Open(ctx)
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
