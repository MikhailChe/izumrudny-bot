package repository

import (
	"testing"
)

func TestUserEventsAreUnique(t *testing.T) {
	eventFQDNs := make(map[string]bool)
	for _, event := range KNOWN_USER_EVENT_TYPES {
		fqdn := event.FQDN()
		if _, ok := eventFQDNs[fqdn]; ok {
			t.Fatalf("Duplicate fqdn: %s", fqdn)
		}
		eventFQDNs[fqdn] = true
	}
}
