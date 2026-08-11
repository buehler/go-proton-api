package proton_test

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-proton-api"
	"github.com/ProtonMail/go-proton-api/server"
	"github.com/go-resty/resty/v2"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEventEndpointVersions(t *testing.T) {
	ctx := t.Context()

	s := server.New()
	defer s.Close()

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	var eventPaths []string
	m.AddPreRequestHook(func(_ *resty.Client, request *resty.Request) error {
		requestURL, err := url.Parse(request.URL)
		if err != nil {
			return err
		}
		if strings.Contains(requestURL.Path, "/events/") {
			eventPaths = append(eventPaths, requestURL.Path)
		}
		return nil
	})

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)
	defer c.Close()

	latestEventID, err := c.GetLatestEventID(ctx)
	require.NoError(t, err)

	_, _, err = c.GetEvent(ctx, latestEventID)
	require.NoError(t, err)

	require.Equal(t, []string{
		"/core/v4/events/latest",
		"/core/v5/events/" + latestEventID,
	}, eventPaths)
}

func TestContactEventDeserialization(t *testing.T) {
	const rawEvent = `{
		"EventID": "event-2",
		"Refresh": 2,
		"Contacts": [
			{
				"ID": "contact-created",
				"Action": 1,
				"Contact": {
					"ID": "contact-created",
					"Name": "Ada Lovelace",
					"UID": "uid-created",
					"Size": 123,
					"CreateTime": 10,
					"ModifyTime": 20,
					"ContactEmails": [{
						"ID": "email-created",
						"Name": "Ada Lovelace",
						"Email": "ada@example.com",
						"Type": ["work"],
						"Defaults": 1,
						"Order": 2,
						"ContactID": "contact-created",
						"CanonicalEmail": "ada@example.com",
						"LabelIDs": ["group-created"],
						"IsProton": 1,
						"LastUsedTime": 42
					}],
					"LabelIDs": ["group-created"],
					"Cards": [{"Type": 3, "Data": "BEGIN:VCARD", "Signature": "signature"}]
				}
			},
			{
				"ID": "contact-updated",
				"Action": 2,
				"Contact": {"ID": "contact-updated", "Name": "Updated", "Cards": []}
			},
			{"ID": "contact-deleted", "Action": 0},
			{"ID": "contact-partial", "Action": 3}
		],
		"ContactEmails": [
			{
				"ID": "email-created",
				"Action": 1,
				"ContactEmail": {
					"ID": "email-created",
					"Name": "Ada Lovelace",
					"Email": "ada@example.com",
					"Defaults": 1,
					"Order": 2,
					"ContactID": "contact-created",
					"CanonicalEmail": "ada@example.com",
					"LabelIDs": ["group-created"],
					"IsProton": 1,
					"LastUsedTime": 42
				}
			},
			{
				"ID": "email-updated",
				"Action": 2,
				"ContactEmail": {
					"ID": "email-updated",
					"ContactID": "contact-updated",
					"CanonicalEmail": null,
					"LabelIDs": [],
					"IsProton": null
				}
			},
			{"ID": "email-deleted", "Action": 0},
			{"ID": "email-partial", "Action": 3}
		],
		"Labels": [
			{
				"ID": "group-created",
				"Action": 1,
				"Label": {
					"ID": "group-created",
					"Name": "Friends",
					"Path": "Friends",
					"Color": "#ff6600",
					"Type": 2
				}
			},
			{"ID": "group-deleted", "Action": 0}
		]
	}`

	var event proton.Event
	require.NoError(t, json.Unmarshal([]byte(rawEvent), &event))

	require.Equal(t, proton.RefreshContacts, event.Refresh)
	require.Equal(t, proton.RefreshFlag(2), proton.RefreshContacts)
	require.Equal(t, proton.EventAction(3), proton.EventPartial)
	require.Equal(t, proton.EventUpdateFlags, proton.EventPartial)

	require.Len(t, event.Contacts, 4)
	require.Equal(t, proton.EventCreate, event.Contacts[0].Action)
	require.NotNil(t, event.Contacts[0].Contact)
	require.Equal(t, "uid-created", event.Contacts[0].Contact.UID)
	require.Len(t, event.Contacts[0].Contact.Cards, 1)
	require.Equal(t, proton.CardType(3), event.Contacts[0].Contact.Cards[0].Type)
	require.Equal(t, proton.EventUpdate, event.Contacts[1].Action)
	require.Nil(t, event.Contacts[2].Contact)
	require.Equal(t, proton.EventDelete, event.Contacts[2].Action)
	require.Nil(t, event.Contacts[3].Contact)
	require.Equal(t, proton.EventPartial, event.Contacts[3].Action)

	embeddedEmail := event.Contacts[0].Contact.ContactEmails[0]
	require.Equal(t, 1, embeddedEmail.Defaults)
	require.Equal(t, 2, embeddedEmail.Order)
	require.Equal(t, "ada@example.com", requireValue(t, embeddedEmail.CanonicalEmail))
	require.True(t, bool(requireValue(t, embeddedEmail.IsProton)))
	require.Equal(t, int64(42), embeddedEmail.LastUsedTime)
	require.Equal(t, []string{"group-created"}, embeddedEmail.LabelIDs)

	require.Len(t, event.ContactEmails, 4)
	require.Equal(t, []string{"group-created"}, event.ContactEmails[0].ContactEmail.LabelIDs)
	require.Nil(t, event.ContactEmails[1].ContactEmail.CanonicalEmail)
	require.Nil(t, event.ContactEmails[1].ContactEmail.IsProton)
	require.Nil(t, event.ContactEmails[2].ContactEmail)
	require.Equal(t, proton.EventDelete, event.ContactEmails[2].Action)
	require.Nil(t, event.ContactEmails[3].ContactEmail)
	require.Equal(t, proton.EventPartial, event.ContactEmails[3].Action)

	require.Len(t, event.Labels, 2)
	require.Equal(t, proton.LabelTypeContactGroup, event.Labels[0].Label.Type)
	require.Equal(t, []string{"Friends"}, event.Labels[0].Label.Path)
	require.Equal(t, "group-deleted", event.Labels[1].ID)
	require.Equal(t, proton.EventDelete, event.Labels[1].Action)
	require.Empty(t, event.Labels[1].Label.ID)

	knownGroupIDs := map[string]struct{}{"group-deleted": {}}
	_, isKnownGroup := knownGroupIDs[event.Labels[1].ID]
	require.True(t, isKnownGroup)

	require.Contains(t, event.String(), "contacts: created=1, updated=2, deleted=1")
	require.Contains(t, event.String(), "contact-emails: created=1, updated=2, deleted=1")
}

func requireValue[T any](t *testing.T, value *T) T {
	t.Helper()
	require.NotNil(t, value)
	return *value
}

func TestEventStreamer(t *testing.T) {
	ctx := t.Context()

	s := server.New()
	defer s.Close()

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)

	createTestMessages(t, c, "pass", 10)

	latestEventID, err := c.GetLatestEventID(ctx)
	require.NoError(t, err)

	eventCh := make(chan proton.Event)

	go func() {
		for event := range c.NewEventStream(ctx, time.Second, 0, latestEventID) {
			eventCh <- event
		}
	}()

	// Perform some action to generate an event.
	metadata, err := c.GetMessageMetadata(ctx, proton.MessageFilter{})
	require.NoError(t, err)
	require.NoError(t, c.LabelMessages(ctx, []string{metadata[0].ID}, proton.TrashLabel))

	// Wait for the first event.
	<-eventCh

	// Close the client; this should stop the client's event streamer.
	c.Close()

	// Create a new client and perform some actions with it to generate more events.
	cc, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)
	defer cc.Close()

	require.NoError(t, cc.LabelMessages(ctx, []string{metadata[1].ID}, proton.TrashLabel))

	// We should not receive any more events from the original client.
	select {
	case <-eventCh:
		require.Fail(t, "received unexpected event")

	default:
		// ...
	}
}

func TestMaxEventMerge(t *testing.T) {
	ctx := t.Context()

	s := server.New()
	defer s.Close()

	s.SetMaxUpdatesPerEvent(1)

	m := proton.New(
		proton.WithHostURL(s.GetHostURL()),
		proton.WithTransport(proton.InsecureTransport()),
	)

	_, _, err := s.CreateUser("user", []byte("pass"))
	require.NoError(t, err)

	c, _, err := m.NewClientWithLogin(ctx, "user", []byte("pass"))
	require.NoError(t, err)

	latestID, err := c.GetLatestEventID(ctx)
	require.NoError(t, err)

	label, err := c.CreateLabel(context.Background(), proton.CreateLabelReq{
		Name:  uuid.NewString(),
		Color: "#f66",
		Type:  proton.LabelTypeFolder,
	})
	require.NoError(t, err)

	for range 75 {
		_, err := c.UpdateLabel(ctx, label.ID, proton.UpdateLabelReq{Name: uuid.NewString()})
		require.NoError(t, err)
	}

	events, more, err := c.GetEvent(ctx, latestID)
	require.NoError(t, err)
	require.True(t, more)
	require.Equal(t, 50, len(events))

	events2, more, err := c.GetEvent(ctx, events[len(events)-1].EventID)
	require.NotEqual(t, events, events2)
	require.NoError(t, err)
	require.False(t, more)
	require.Equal(t, 26, len(events2))
}
