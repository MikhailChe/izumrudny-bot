package repository

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestUserEventsAreUnique(t *testing.T) {
	eventFQDNs := make(map[string]bool)
	for _, event := range knownUserEventTypes {
		fqdn := event.FQDN()
		if _, ok := eventFQDNs[fqdn]; ok {
			t.Fatalf("Duplicate fqdn: %s", fqdn)
		}
		eventFQDNs[fqdn] = true
	}
}

func TestSelectTypeWorks(t *testing.T) {
	var ctx = context.Background()
	for _, event := range knownUserEventTypes {
		t.Run(event.FQDN(), func(t *testing.T) {
			selectedEvent := SelectType(ctx, event.FQDN())
			if selectedEvent == nil {
				t.Fatalf("Cannot select event by type fqdn: %T", selectedEvent)
			}
		})
	}
	if event := SelectType(ctx, "unknownEvent"); event != nil {
		t.Fatalf("should have retrurned nil, but returned: %T", event)
	}
}

func TestApplyUserEvents(t *testing.T) {
	type args struct {
		events []UserEvent
	}
	type userChecker func(u User) error
	type tSubtest struct {
		args      args
		validator userChecker
	}
	checkRegistrationStarted := func(u User) error {
		if u.Registration == nil {
			return fmt.Errorf("expected legacy registration struct to exist, got nil")
		}
		if u.Registration.Events.Start == nil {
			return fmt.Errorf("expected legacy registration start event to exist, got nil")
		}
		if len(u.Apartments) > 0 {
			return fmt.Errorf("expected list of apartments (legacy) to be empty after start of registration, got: %#v", u.Apartments)
		}
		if len(u.PrivateProperty.Items) != 1 {
			return fmt.Errorf("expected list of PrivateProperty to containt an appartment, got: %#v", u.PrivateProperty.Items)
		}
		for _, value := range u.PrivateProperty.Items {
			if value.Approved {
				return fmt.Errorf("expected private property not to be approved, got: %#v", value)
			}
		}
		return nil
	}
	carPlateChecker := func(u User) error {
		if len(u.Cars) != 1 {
			return fmt.Errorf("expected cars slice to contain licence plate, got: %#v", u.Cars)
		}
		if u.Cars[0].LicensePlate != "X703BX96" {
			return fmt.Errorf("expected X703BX96, got: #%v", u.Cars[0].LicensePlate)
		}
		return nil
	}
	var subtests = map[string]tSubtest{
		"StartRegistrationEvent": {
			args: args{events: []UserEvent{&StartRegistrationEvent{
				UpdateID:     123,
				HouseNumber:  "108Г",
				HouseID:      4,
				Apartment:    "3",
				ApproveCode:  "3А2СХ",
				InvalidCodes: nil,
			}}},
			validator: checkRegistrationStarted,
		},
		"StartRegistrationEventIdempotent": {
			args: args{events: []UserEvent{&StartRegistrationEvent{
				UpdateID:     123,
				HouseNumber:  "108Г",
				HouseID:      4,
				Apartment:    "3",
				ApproveCode:  "3А2СХ",
				InvalidCodes: nil,
			}, &StartRegistrationEvent{
				UpdateID:     123,
				HouseNumber:  "108Г",
				HouseID:      4,
				Apartment:    "3",
				ApproveCode:  "3А2СХ",
				InvalidCodes: nil,
			}}},
			validator: checkRegistrationStarted,
		},
		"ConfirmRegistrationEvent": {
			args: args{events: []UserEvent{&ConfirmRegistrationEvent{
				UpdateID: 777,
				WithCode: "квитанция",
			}}},
			validator: func(u User) error {
				if len(u.Apartments) > 0 {
					return fmt.Errorf("expected list of apartments (legacy) to be empty after start of registration, got: %#v", u.Apartments)
				}
				if len(u.PrivateProperty.Items) > 0 {
					return fmt.Errorf("expected list of PrivateProperty to containt an appartment, got: %#v", u.PrivateProperty.Items)
				}
				if u.Registration != nil {
					return fmt.Errorf("expected not to have registration")
				}
				return nil
			},
		},
		"FailRegistrationEvent": {
			args: args{events: []UserEvent{&FailRegistrationEvent{
				UpdateID: 777,
				WithCode: "квитанция",
			}}},
			validator: func(u User) error {
				if len(u.Apartments) > 0 {
					return fmt.Errorf("expected list of apartments (legacy) to be empty after start of registration, got: %#v", u.Apartments)
				}
				if len(u.PrivateProperty.Items) > 0 {
					return fmt.Errorf("expected list of PrivateProperty to containt an appartment, got: %#v", u.PrivateProperty.Items)
				}
				if u.Registration != nil {
					return fmt.Errorf("expected not to have registration")
				}
				return nil
			},
		},
		"StartAndConfirmRegistrationEvent": {
			args: args{events: []UserEvent{
				&StartRegistrationEvent{
					UpdateID:     123,
					HouseNumber:  "108Г",
					HouseID:      4,
					Apartment:    "3",
					ApproveCode:  "3А2СХ",
					InvalidCodes: nil,
				},
				&ConfirmRegistrationEvent{
					UpdateID: 777,
					WithCode: "квитанция",
				},
			}},
			validator: func(u User) error {
				if len(u.Apartments) != 1 {
					return fmt.Errorf("expected list of apartments (legacy) to contain apartment, got: %#v", u.Apartments)
				}
				expectedApartment := Apartment{HouseNumber: "108Г", HouseID: 4, ApartmentNumber: "3", NeedApprove: false}
				if u.Apartments[0] != expectedApartment {
					return fmt.Errorf("expected apartment to be %#v, got %#v", expectedApartment, u.Apartments[0])
				}
				if len(u.PrivateProperty.Items) != 1 {
					return fmt.Errorf("expected list of PrivateProperty to containt an appartment, got: %#v", u.PrivateProperty.Items)
				}
				if u.Registration != nil {
					return fmt.Errorf("expected not to have registration")
				}
				return nil
			},
		},
		"StartAndFailRegistrationEvent": {
			args: args{events: []UserEvent{
				&StartRegistrationEvent{
					UpdateID:     123,
					HouseNumber:  "108Г",
					HouseID:      4,
					Apartment:    "3",
					ApproveCode:  "3А2СХ",
					InvalidCodes: nil,
				},
				&FailRegistrationEvent{
					UpdateID: 777,
					WithCode: "квитанция",
				},
			}},
			validator: func(u User) error {
				if len(u.Apartments) != 0 {
					return fmt.Errorf("expected list of apartments (legacy) to be empty, got: %#v", u.Apartments)
				}
				if len(u.PrivateProperty.Items) != 0 {
					return fmt.Errorf("expected list of PrivateProperty to be empty, got: %#v", u.PrivateProperty.Items)
				}
				if u.Registration != nil {
					return fmt.Errorf("expected not to have registration")
				}
				return nil
			},
		},
		"RegisterCarLicensePlateEvent": {
			args: args{events: []UserEvent{&RegisterCarLicensePlateEvent{
				UpdateID:     0,
				LicensePlate: "X703BX96",
			}}},
			validator: carPlateChecker,
		},
		"RegisterCarLicensePlateEventIdempotent": {
			args: args{events: []UserEvent{&RegisterCarLicensePlateEvent{
				UpdateID:     0,
				LicensePlate: "X703BX96",
			}, &RegisterCarLicensePlateEvent{
				UpdateID:     1,
				LicensePlate: "X703BX96",
			}}},
			validator: carPlateChecker,
		},
		"AddApartmentEventV2": {
			args: args{events: []UserEvent{&AddApartmentEventV2{
				UpdateID:     12345,
				HouseID:      4,
				Apartment:    "3",
				ApproveCode:  "",
				InvalidCodes: nil,
			}}},
			validator: func(u User) error {
				if len(u.Apartments) != 0 {
					return fmt.Errorf("ожидается, что v2 не заполняет Apartments, получил %#v", u.Apartments)
				}
				if u.Registration != nil {
					return fmt.Errorf("ожидается, что v2 не заполняет поле Registration, получил %#v", u.Registration)
				}
				if len(u.PrivateProperty.Items) != 1 {
					return fmt.Errorf("ожидается, что v2 заполняет частную собственность, получил %#v", u.PrivateProperty.Items)
				}
				return nil
			},
		},
		"AdminConfirmedAddApartmentEventV2": {
			args: args{events: []UserEvent{&AdminConfirmedAddApartmentEventV2{
				AdminUserID: 78225,
				HouseID:     4,
				Apartment:   "3",
			}}},
			validator: func(u User) error {
				if len(u.Apartments) != 0 {
					return fmt.Errorf("ожидал пусктой список апартаментов, но получил: %#v", u.Apartments)
				}
				if u.Registration != nil {
					return fmt.Errorf("ожидал, что поле регистрации не будет заполнено, потому что для новых событий регистрация формируется через поля в PrivateProperty, получил %#v", u.Registration)
				}
				if len(u.PrivateProperty.Items) != 0 {
					return fmt.Errorf("ожидается, что при наличии подтверждения без запроса - ничего добавляться не дожно, но получил %#v", u.PrivateProperty.Items)
				}
				return nil
			},
		},
		"AdminDeclinedAddApartmentEventV2": {
			args: args{events: []UserEvent{&AdminDeclinedAddApartmentEventV2{
				AdminUserID: 78225,
				HouseID:     4,
				Apartment:   "3",
			}}},
			validator: func(u User) error {
				if len(u.Apartments) != 0 {
					return fmt.Errorf("ожидал пусктой список апартаментов, но получил: %#v", u.Apartments)
				}
				if u.Registration != nil {
					return fmt.Errorf("ожидал, что поле регистрации не будет заполнено, потому что для новых событий регистрация формируется через поля в PrivateProperty, получил %#v", u.Registration)
				}
				if len(u.PrivateProperty.Items) != 0 {
					return fmt.Errorf("ожидается, что при отклонении запроса всё должно быть пустым, но получил %#v", u.PrivateProperty.Items)
				}
				return nil
			},
		},
		"AdminDeclinedAddApartmentEventV2AfterAdd": {
			args: args{events: []UserEvent{&AdminConfirmedAddApartmentEventV2{
				AdminUserID: 78225,
				HouseID:     4,
				Apartment:   "3",
			}, &AdminDeclinedAddApartmentEventV2{
				AdminUserID: 78225,
				HouseID:     4,
				Apartment:   "3",
			}}},
			validator: func(u User) error {
				if len(u.Apartments) != 0 {
					return fmt.Errorf("ожидал пусктой список апартаментов, но получил: %#v", u.Apartments)
				}
				if u.Registration != nil {
					return fmt.Errorf("ожидал, что поле регистрации не будет заполнено, потому что для новых событий регистрация формируется через поля в PrivateProperty, получил %#v", u.Registration)
				}
				if len(u.PrivateProperty.Items) != 0 {
					return fmt.Errorf("ожидается, что при отклонении запроса всё должно быть пустым, но получил %#v", u.PrivateProperty.Items)
				}
				return nil
			},
		},
		"AddAndApproveV2": {
			args: args{events: []UserEvent{&AddApartmentEventV2{
				UpdateID:     12345,
				HouseID:      4,
				Apartment:    "3",
				ApproveCode:  "",
				InvalidCodes: nil,
			}, &AdminConfirmedAddApartmentEventV2{
				AdminUserID: 78225,
				HouseID:     4,
				Apartment:   "3",
			}}},
			validator: func(u User) error {
				if len(u.Apartments) != 0 {
					return fmt.Errorf("ожидал пусктой список апартаментов, но получил: %#v", u.Apartments)
				}
				if u.Registration != nil {
					return fmt.Errorf("ожидал, что поле регистрации не будет заполнено, потому что для новых событий регистрация формируется через поля в PrivateProperty, получил %#v", u.Registration)
				}
				if len(u.PrivateProperty.Items) == 0 {
					return fmt.Errorf("ожидается, что при отклонении подтвержденного запроса всё должно остаться как есть, но получил %#v", u.PrivateProperty.Items)
				}
				return nil
			},
		},
		"AdminDeclinedAddApartmentEventV2AfterAddAndApprove": {
			args: args{events: []UserEvent{&AddApartmentEventV2{
				UpdateID:     12345,
				HouseID:      4,
				Apartment:    "3",
				ApproveCode:  "",
				InvalidCodes: nil,
			}, &AdminConfirmedAddApartmentEventV2{
				AdminUserID: 78225,
				HouseID:     4,
				Apartment:   "3",
			}, &AdminDeclinedAddApartmentEventV2{
				AdminUserID: 78225,
				HouseID:     4,
				Apartment:   "3",
			}}},
			validator: func(u User) error {
				if len(u.Apartments) != 0 {
					return fmt.Errorf("ожидал пусктой список апартаментов, но получил: %#v", u.Apartments)
				}
				if u.Registration != nil {
					return fmt.Errorf("ожидал, что поле регистрации не будет заполнено, потому что для новых событий регистрация формируется через поля в PrivateProperty, получил %#v", u.Registration)
				}
				if len(u.PrivateProperty.Items) == 0 {
					return fmt.Errorf("ожидается, что при отклонении подтвержденного запроса всё должно остаться как есть, но получил %#v", u.PrivateProperty.Items)
				}
				return nil
			},
		},
	}

	for name, subtest := range subtests {
		t.Run(name, func(subtest tSubtest) func(t *testing.T) {
			return func(t *testing.T) {
				var user User
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
				defer cancel()
				for _, event := range subtest.args.events {
					event.Apply(ctx, &user)
				}
				if err := subtest.validator(user); err != nil {
					t.Fatalf("user does not match expectation: %v", err)
				}
			}
		}(subtest))
	}
}
