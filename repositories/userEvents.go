package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"mikhailche/botcomod/tracer"
	"reflect"
	"time"

	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

type UserEvent interface {
	Apply(*User)
	FQDN() string
}

type StartRegistrationEvent struct {
	UpdateID     int64
	HouseNumber  string
	Apartment    string `json:"Appartment"`
	ApproveCode  string
	InvalidCodes []string
}

func (e *StartRegistrationEvent) Apply(u *User) {
	defer tracer.Trace("startRegistrationEvent::Apply")()
	u.Registration = &tRegistration{
		Events: tRegistrationEvents{Start: e},
	}
}

func (e *StartRegistrationEvent) FQDN() string {
	return "*bot.startRegistrationEvent"
}

type ConfirmRegistrationEvent struct {
	UpdateID int64
	WithCode string
}

func (e *ConfirmRegistrationEvent) Apply(u *User) {
	defer tracer.Trace("confirmRegistrationEvent::Apply")()
	u.Apartments = append(u.Apartments, Apartment{
		HouseNumber:     u.Registration.Events.Start.HouseNumber,
		ApartmentNumber: u.Registration.Events.Start.Apartment,
		NeedApprove:     false,
	})
	u.IsApprovedResident = true
	u.Registration = nil
}

func (e *ConfirmRegistrationEvent) FQDN() string {
	return "*bot.confirmRegistrationEvent"
}

type FailRegistrationEvent struct {
	UpdateID int64
	WithCode string
}

func (e *FailRegistrationEvent) Apply(u *User) {
	defer tracer.Trace("failRegistrationEvent::Apply")()
	u.Registration = nil
}

func (e *FailRegistrationEvent) FQDN() string {
	return "*bot.failRegistrationEvent"
}

type RegisterCarLicensePlateEvent struct {
	UpdateID     int64
	LicensePlate string
}

func (e *RegisterCarLicensePlateEvent) Apply(u *User) {
	defer tracer.Trace("registerCarLicensePlateEvent::Apply")()
	u.Cars = append(u.Cars, Car{LicensePlate: e.LicensePlate})
}

func (e *RegisterCarLicensePlateEvent) FQDN() string {
	return "*bot.registerCarLicensePlateEvent"
}

var KNOWN_USER_EVENT_TYPES = [...]UserEvent{
	((*StartRegistrationEvent)(nil)),
	((*ConfirmRegistrationEvent)(nil)),
	((*FailRegistrationEvent)((nil))),
	((*RegisterCarLicensePlateEvent)((nil))),
}

func SelectType(typeName string) UserEvent {
	defer tracer.Trace("SelectType")()
	for _, t := range KNOWN_USER_EVENT_TYPES {
		if t.FQDN() == typeName {
			return reflect.New(reflect.TypeOf(t).Elem()).Interface().(UserEvent)
		}
	}
	return nil
}

func (r *UserRepository) LogEvent(ctx context.Context, userID int64, event UserEvent) error {
	defer tracer.Trace("UserRepository::LogEvent")()
	now := time.Now()
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
				table.ValueParam("$type", types.StringValueFromString(fmt.Sprintf("%T", event))),
				table.ValueParam("$event", types.JSONDocumentValueFromBytes(eventBytes)),
			),
		)
		if err != nil {
			return fmt.Errorf("upsert user_event: %w", err)
		}
		return nil
	}, table.WithIdempotent())
}
