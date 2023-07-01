package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	. "mikhailche/botcomod/tracer"

	"github.com/ydb-platform/ydb-go-sdk/v3/table"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
)

type UserEvent interface {
	Apply(*User)
}

type startRegistrationEvent struct {
	UpdateID     int64
	HouseNumber  string
	Appartment   string
	ApproveCode  string
	InvalidCodes []string
}

func (e *startRegistrationEvent) Apply(u *User) {
	defer Trace("startRegistrationEvent::Apply")()
	u.Registration = &tRegistration{
		Events: tRegistrationEvents{Start: e},
	}
}

type confirmRegistrationEvent struct {
	UpdateID int64
	WithCode string
}

func (e *confirmRegistrationEvent) Apply(u *User) {
	defer Trace("confirmRegistrationEvent::Apply")()
	u.Appartments = append(u.Appartments, Appart{
		HouseNumber:      u.Registration.Events.Start.HouseNumber,
		AppartmentNumber: u.Registration.Events.Start.Appartment,
		NeedApprove:      false,
	})
	u.IsApprovedResident = true
	u.Registration = nil
}

type failRegistrationEvent struct {
	UpdateID int64
	WithCode string
}

func (e *failRegistrationEvent) Apply(u *User) {
	defer Trace("failRegistrationEvent::Apply")()
	u.Registration = nil
}

type registerCarLicensePlateEvent struct {
	UpdateID     int64
	LicensePlate string
}

func (e *registerCarLicensePlateEvent) Apply(u *User) {
	defer Trace("registerCarLicensePlateEvent::Apply")()
	u.Cars = append(u.Cars, Car{LicensePlate: e.LicensePlate})
}

var KNOWN_USER_EVENT_TYPES = [...]UserEvent{
	((*startRegistrationEvent)(nil)),
	((*confirmRegistrationEvent)(nil)),
	((*failRegistrationEvent)((nil))),
	((*registerCarLicensePlateEvent)((nil))),
}

func SelectType(typeName string) UserEvent {
	defer Trace("SelectType")()
	for _, t := range KNOWN_USER_EVENT_TYPES {
		if fmt.Sprintf("%T", t) == typeName {
			return reflect.New(reflect.TypeOf(t).Elem()).Interface().(UserEvent)
		}
	}
	return nil
}

func (r *UserRepository) LogEvent(ctx context.Context, userID int64, event UserEvent) error {
	defer Trace("UserRepository::LogEvent")()
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
