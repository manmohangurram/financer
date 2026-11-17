package mail

import (
	"fmt"
	"sync"
	"time"

	"github.com/emersion/go-imap/v2"
	"github.com/emersion/go-imap/v2/imapclient"

	apierrors "github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/repo"
)

// IMAPSource reads a mailbox over implicit TLS (port 993).
type IMAPSource struct {
	mu     sync.Mutex
	client *imapclient.Client
}

// ConnectIMAP opens an IMAP session for the account.
func ConnectIMAP(account *repo.MailAccountRow, password string) (*IMAPSource, error) {
	host := account.IMAPHost
	addr := fmt.Sprintf("%s:%d", host, account.IMAPPort)

	c, err := imapclient.DialTLS(addr, nil)
	if err != nil {
		return nil, apierrors.BadGateway(fmt.Sprintf("imap TLS to %s failed: %v", addr, err))
	}
	if err := c.Login(account.Username, password).Wait(); err != nil {
		_ = c.Close()
		return nil, apierrors.BadRequest(fmt.Sprintf("imap login failed: %v", err))
	}
	if _, err := c.Select("INBOX", nil).Wait(); err != nil {
		_ = c.Close()
		return nil, apierrors.BadGateway(fmt.Sprintf("imap SELECT INBOX failed: %v", err))
	}
	return &IMAPSource{client: c}, nil
}

// Close logs out and closes the session.
func (s *IMAPSource) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return nil
	}
	_ = s.client.Logout().Wait()
	err := s.client.Close()
	s.client = nil
	return err
}

// Headers returns the recent INBOX envelopes (all senders; rules decide what
// matters). The window is bounded to avoid pulling an entire mailbox.
func (s *IMAPSource) Headers(sinceDays int) ([]MailHeader, error) {
	const maxMessages = 200
	since := time.Now().UTC().AddDate(0, 0, -sinceDays)

	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return nil, apierrors.BadGateway("imap session closed")
	}

	data, err := s.client.UIDSearch(&imap.SearchCriteria{Since: since}, nil).Wait()
	if err != nil {
		return nil, apierrors.BadGateway(fmt.Sprintf("imap SEARCH failed: %v", err))
	}
	all := data.AllUIDs()
	if len(all) == 0 {
		return nil, nil
	}
	if len(all) > maxMessages {
		all = all[len(all)-maxMessages:]
	}
	uids := make([]imap.UID, 0, len(all))
	for _, uid := range all {
		uids = append(uids, imap.UID(uid))
	}

	msgs, err := s.client.Fetch(imap.UIDSetNum(uids...), &imap.FetchOptions{
		UID:      true,
		Envelope: true,
	}).Collect()
	if err != nil {
		return nil, apierrors.BadGateway(fmt.Sprintf("imap FETCH headers failed: %v", err))
	}

	out := make([]MailHeader, 0, len(msgs))
	for _, msg := range msgs {
		// No Message-ID means no idempotency key, so the message is skipped.
		if msg.Envelope == nil || msg.Envelope.MessageID == "" {
			continue
		}
		from := ""
		if len(msg.Envelope.From) > 0 {
			from = msg.Envelope.From[0].Addr()
		}
		var receivedAt *string
		if !msg.Envelope.Date.IsZero() {
			v := msg.Envelope.Date.Format(time.RFC3339)
			receivedAt = &v
		}
		out = append(out, MailHeader{
			UID:        uint32(msg.UID),
			MessageID:  msg.Envelope.MessageID,
			FromAddr:   from,
			Subject:    msg.Envelope.Subject,
			ReceivedAt: receivedAt,
		})
	}
	return out, nil
}

// Body returns the raw message (headers included), which is what the template
// extractor scans.
func (s *IMAPSource) Body(uid uint32) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.client == nil {
		return "", apierrors.BadGateway("imap session closed")
	}
	msgs, err := s.client.Fetch(imap.UIDSetNum(imap.UID(uid)), &imap.FetchOptions{
		UID: true,
		BodySection: []*imap.FetchItemBodySection{{
			Peek: true,
		}},
	}).Collect()
	if err != nil {
		return "", apierrors.BadGateway(fmt.Sprintf("imap FETCH body failed: %v", err))
	}
	for _, msg := range msgs {
		for _, section := range msg.BodySection {
			return string(section.Bytes), nil
		}
	}
	return "", apierrors.BadGateway("imap returned no body")
}
