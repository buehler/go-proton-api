package proton

import (
	"fmt"
	"strings"

	"github.com/bradenaw/juniper/xslices"
)

type Event struct {
	EventID string

	Refresh RefreshFlag

	User *User

	UserSettings *UserSettings

	MailSettings *MailSettings

	Messages []MessageEvent

	Contacts []ContactEvent

	ContactEmails []ContactEmailEvent

	Labels []LabelEvent

	Addresses []AddressEvent

	Notifications []NotificationEvent

	UsedSpace *int64
}

func (event Event) String() string {
	var parts []string

	if event.Refresh != 0 {
		parts = append(parts, fmt.Sprintf("refresh: %v", event.Refresh))
	}

	if event.User != nil {
		parts = append(parts, "user: [modified]")
	}

	if event.MailSettings != nil {
		parts = append(parts, "mail-settings: [modified]")
	}

	if len(event.Messages) > 0 {
		parts = append(parts, fmt.Sprintf(
			"messages: created=%d, updated=%d, deleted=%d",
			xslices.CountFunc(event.Messages, func(e MessageEvent) bool { return e.Action == EventCreate }),
			xslices.CountFunc(event.Messages, func(e MessageEvent) bool { return e.Action == EventUpdate || e.Action == EventUpdateFlags }),
			xslices.CountFunc(event.Messages, func(e MessageEvent) bool { return e.Action == EventDelete }),
		))
	}

	if len(event.Contacts) > 0 {
		parts = append(parts, fmt.Sprintf(
			"contacts: created=%d, updated=%d, deleted=%d",
			xslices.CountFunc(event.Contacts, func(e ContactEvent) bool { return e.Action == EventCreate }),
			xslices.CountFunc(event.Contacts, func(e ContactEvent) bool { return e.Action == EventUpdate || e.Action == EventPartial }),
			xslices.CountFunc(event.Contacts, func(e ContactEvent) bool { return e.Action == EventDelete }),
		))
	}

	if len(event.ContactEmails) > 0 {
		parts = append(parts, fmt.Sprintf(
			"contact-emails: created=%d, updated=%d, deleted=%d",
			xslices.CountFunc(event.ContactEmails, func(e ContactEmailEvent) bool { return e.Action == EventCreate }),
			xslices.CountFunc(event.ContactEmails, func(e ContactEmailEvent) bool { return e.Action == EventUpdate || e.Action == EventPartial }),
			xslices.CountFunc(event.ContactEmails, func(e ContactEmailEvent) bool { return e.Action == EventDelete }),
		))
	}

	if len(event.Labels) > 0 {
		parts = append(parts, fmt.Sprintf(
			"labels: created=%d, updated=%d, deleted=%d",
			xslices.CountFunc(event.Labels, func(e LabelEvent) bool { return e.Action == EventCreate }),
			xslices.CountFunc(event.Labels, func(e LabelEvent) bool { return e.Action == EventUpdate || e.Action == EventUpdateFlags }),
			xslices.CountFunc(event.Labels, func(e LabelEvent) bool { return e.Action == EventDelete }),
		))
	}

	if len(event.Addresses) > 0 {
		parts = append(parts, fmt.Sprintf(
			"addresses: created=%d, updated=%d, deleted=%d",
			xslices.CountFunc(event.Addresses, func(e AddressEvent) bool { return e.Action == EventCreate }),
			xslices.CountFunc(event.Addresses, func(e AddressEvent) bool { return e.Action == EventUpdate || e.Action == EventUpdateFlags }),
			xslices.CountFunc(event.Addresses, func(e AddressEvent) bool { return e.Action == EventDelete }),
		))
	}

	return fmt.Sprintf("Event %s: %s", event.EventID, strings.Join(parts, ", "))
}

type RefreshFlag uint8

const (
	RefreshMail     RefreshFlag = 1 << iota   // 1<<0 = 1
	RefreshContacts                           // 1<<1 = 2
	_                                         // 1<<2 = 4
	_                                         // 1<<3 = 8
	_                                         // 1<<4 = 16
	_                                         // 1<<5 = 32
	_                                         // 1<<6 = 64
	_                                         // 1<<7 = 128
	RefreshAll      RefreshFlag = 1<<iota - 1 // 1<<8 - 1 = 255
)

type EventAction int

const (
	EventDelete EventAction = iota
	EventCreate
	EventUpdate
	EventUpdateFlags

	// EventPartial indicates that the event payload may be incomplete. Consumers
	// should fetch the entity before replacing their local copy.
	EventPartial EventAction = EventUpdateFlags
)

type EventItem struct {
	ID     string
	Action EventAction
}

type MessageEvent struct {
	EventItem

	Message MessageMetadata
}

type ContactEvent struct {
	EventItem

	// Contact can be absent for delete and partial events.
	Contact *Contact
}

type ContactEmailEvent struct {
	EventItem

	// ContactEmail can be absent for delete and partial events. LabelIDs contains
	// the contact-group memberships when the payload is present.
	ContactEmail *ContactEmail
}

type LabelEvent struct {
	EventItem

	// Contact groups are labels with Type == LabelTypeContactGroup. Delete
	// events may omit Label, so consumers must classify known group IDs locally.
	Label Label
}

type AddressEvent struct {
	EventItem

	Address Address
}
