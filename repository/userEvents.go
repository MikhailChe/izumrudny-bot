package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/result"
	"mikhailche/botcomod/lib/errors"
	"mikhailche/botcomod/lib/tracer.v2"
	"reflect"
	"time"

	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

type UserEvent interface {
	Apply(context.Context, *User)
	FQDN() string
}

type StartRegistrationEvent struct {
	UpdateID     int64
	HouseNumber  string
	HouseID      uint64
	Apartment    string `json:"Appartment"`
	ApproveCode  string
	InvalidCodes []string
}

type ConfirmRegistrationEvent struct {
	UpdateID int64
	WithCode string
}

type FailRegistrationEvent struct {
	UpdateID int64
	WithCode string
}

type RegisterCarLicensePlateEvent struct {
	UpdateID     int64
	LicensePlate string
}

// AddApartmentEventV2 вторая версия [StartRegistrationEvent].
// Флоу регистрации подразумевает последовательное добавление неограниченного количества квартир
// Можно будет выделить два уровня подтверждения резиденства: купили квартиру и приняли квартиру.
// Любой из них можно будет подтвердить
type AddApartmentEventV2 struct {
	UpdateID int64
	// HouseId - идентификатор дома в YDB таблице house
	HouseID      uint64
	Apartment    string
	ApproveCode  string
	InvalidCodes []string
}

// AdminConfirmedAddApartmentEventV2 Событие означает, что администратор подтвердил резиденство от [AddApartmentEventV2]
type AdminConfirmedAddApartmentEventV2 struct {
	AdminUserID int64
	HouseID     uint64
	Apartment   string
}

// AdminDeclinedAddApartmentEventV2 Событие означает, что администратор отверг запрос на резиденство от [AddApartmentEventV2]
type AdminDeclinedAddApartmentEventV2 struct {
	AdminUserID int64
	HouseID     uint64
	Apartment   string
	Reason      string
}

func (e *StartRegistrationEvent) Apply(ctx context.Context, u *User) {
	ctx, span := tracer.Open(ctx, tracer.Named("startRegistrationEvent::Apply"))
	defer span.Close()
	u.Registration = &tRegistration{
		Events: tRegistrationEvents{Start: e},
	}
}

func (e *ConfirmRegistrationEvent) Apply(ctx context.Context, u *User) {
	ctx, span := tracer.Open(ctx, tracer.Named("confirmRegistrationEvent::Apply"))
	defer span.Close()
	u.Apartments = append(u.Apartments, Apartment{
		HouseNumber:     u.Registration.Events.Start.HouseNumber,
		ApartmentNumber: u.Registration.Events.Start.Apartment,
		NeedApprove:     false,
	})
	u.IsApprovedResident = true
	u.Registration = nil
}

func (e *FailRegistrationEvent) Apply(ctx context.Context, u *User) {
	ctx, span := tracer.Open(ctx, tracer.Named("failRegistrationEvent::Apply"))
	defer span.Close()
	u.Registration = nil
}

func (e *RegisterCarLicensePlateEvent) Apply(ctx context.Context, u *User) {
	ctx, span := tracer.Open(ctx, tracer.Named("registerCarLicensePlateEvent::Apply"))
	defer span.Close()
	for _, car := range u.Cars {
		if car.LicensePlate == e.LicensePlate {
			return
		}
	}
	u.Cars = append(u.Cars, Car{LicensePlate: e.LicensePlate})
}

func (a *AddApartmentEventV2) Apply(ctx context.Context, user *User) {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	user.PrivateProperty.Add(a.HouseID, a.Apartment)
}

func (a *AdminConfirmedAddApartmentEventV2) Apply(ctx context.Context, user *User) {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	user.PrivateProperty.Approve(a.HouseID, a.Apartment)
}

func (a *AdminDeclinedAddApartmentEventV2) Apply(ctx context.Context, user *User) {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	user.PrivateProperty.RemoveIfNotApproved(a.HouseID, a.Apartment)
}

func (e *StartRegistrationEvent) FQDN() string {
	return "*bot.startRegistrationEvent"
}
func (e *ConfirmRegistrationEvent) FQDN() string {
	return "*bot.confirmRegistrationEvent"
}
func (e *FailRegistrationEvent) FQDN() string {
	return "*bot.failRegistrationEvent"
}
func (e *RegisterCarLicensePlateEvent) FQDN() string {
	return "*bot.registerCarLicensePlateEvent"
}
func (a *AddApartmentEventV2) FQDN() string {
	return "AddApartmentEventV2"
}
func (a *AdminConfirmedAddApartmentEventV2) FQDN() string {
	return "AdminConfirmedAddApartmentEventV2"
}
func (a *AdminDeclinedAddApartmentEventV2) FQDN() string {
	return "AdminDeclinedAddApartmentEventV2"
}

var knownUserEventTypes = [...]UserEvent{
	(*StartRegistrationEvent)(nil),
	(*ConfirmRegistrationEvent)(nil),
	(*FailRegistrationEvent)(nil),
	(*RegisterCarLicensePlateEvent)(nil),
	(*AddApartmentEventV2)(nil),
	(*AdminConfirmedAddApartmentEventV2)(nil),
	(*AdminDeclinedAddApartmentEventV2)(nil),
}

func SelectType(ctx context.Context, typeName string) UserEvent {
	ctx, span := tracer.Open(ctx, tracer.Named("SelectType"))
	defer span.Close()
	for _, t := range knownUserEventTypes {
		if t.FQDN() == typeName {
			return reflect.New(reflect.TypeOf(t).Elem()).Interface().(UserEvent)
		}
	}
	return nil
}

func (r *UserRepository) LogEvent(ctx context.Context, userID int64, event UserEvent, logEventOptions ...any) error {
	ctx, span := tracer.Open(ctx, tracer.Named("UserRepository::LogEvent"))
	defer span.Close()
	now := time.Now()
	for _, opt := range logEventOptions {
		if timeFn, ok := opt.(func() time.Time); ok {
			now = timeFn()
		}
	}
	eventBytes, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("сериализация события %v: %w", event, err)
	}
	return (*r.DB).Table().Do(ctx, func(ctx context.Context, s table.Session) error {
		_, _, err := s.Execute(
			ctx,
			table.DefaultTxControl(),
			"DECLARE $user AS Int64;"+
				"DECLARE $timestamp AS Timestamp;"+
				"DECLARE $type AS String;"+
				"DECLARE $event AS JsonDocument;"+
				"UPSERT INTO `user_event` (user, timestamp, id, type, event)"+
				"VALUES ($user, $timestamp, CAST(RandomUUID($timestamp) AS String), $type, $event);",
			table.NewQueryParameters(
				table.ValueParam("$user", types.Int64Value(userID)),
				table.ValueParam("$timestamp", types.TimestampValueFromTime(now)),
				table.ValueParam("$type", types.StringValueFromString(event.FQDN())),
				table.ValueParam("$event", types.JSONDocumentValueFromBytes(eventBytes)),
			),
		)
		if err != nil {
			return fmt.Errorf("upsert user_event: %w", err)
		}
		return nil
	}, table.WithIdempotent())
}

func (r *UserRepository) MigrateEvents(ctx context.Context, s table.Session, houseIdByHouseNumber func(number string) (uint64, error)) error {
	ctx, span := tracer.Open(ctx)
	defer span.Close()
	getAllBotStartRegistrationEventRecords := func() ([]UserEventRecord, error) {
		var eventRecords []UserEventRecord
		_, res, err := s.Execute(ctx, table.DefaultTxControl(),
			`DECLARE $id AS Int64;
SELECT * FROM user_event WHERE type = "*bot.startRegistrationEvent";`,
			table.NewQueryParameters(),
		)
		if err != nil {
			return nil, fmt.Errorf("SELECT * FROM user_event: %w", err)
		}
		defer func(res result.Result) {
			_ = res.Close()
		}(res)
		if !res.NextResultSet(ctx) {
			return nil, fmt.Errorf("не нашел result set для событий пользователя; невалидный запрос?")
		}
		for res.NextRow() {
			var eventRecord UserEventRecord
			if err := eventRecord.Scan(ctx, res); err != nil {
				return nil, fmt.Errorf("не смог события пользователя: %w", err)
			}
			if _, ok := eventRecord.Event.(*StartRegistrationEvent); ok {
				eventRecords = append(eventRecords, eventRecord)
			}
		}
		return eventRecords, errors.ErrorfOrNil(res.Err(), "applyEvents")
	}

	allEventRecords, err := getAllBotStartRegistrationEventRecords()
	if err != nil {
		return fmt.Errorf("MigrateEvents: %w", err)
	}
	for _, record := range allEventRecords {
		event := record.Event.(*StartRegistrationEvent)
		event.HouseID, err = houseIdByHouseNumber(event.HouseNumber)
		if err != nil {
			return fmt.Errorf("MigrateEvents: %w", err)
		}
		err := r.LogEvent(ctx, record.User, event, func() time.Time { return record.Timestamp })
		if err != nil {
			return fmt.Errorf("MigrateEvents: %w", err)
		}
	}
	return nil
}
