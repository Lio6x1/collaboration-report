package event

import (
	"bytes"
	domain "collaboration/search/internal/domain/event"
	"crypto/sha256"
	"errors"
	"regexp"
)

type Position = domain.Position

var consumerName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,39}$`)
var topicName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,159}$`)
var physicalName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,159}$`)
var errorCode = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)

func validatePosition(consumer string, position Position) error {
	if !consumerName.MatchString(consumer) || !topicName.MatchString(position.Topic) || position.Partition < 0 || position.Offset < 0 {
		return errors.New("SEARCH_MESSAGE_POSITION_INVALID")
	}
	return nil
}

func validateEvent(value domain.Event) error {
	stored, err := domain.RestoreEvent(value.Original)
	if err != nil || sha256.Sum256(value.Original) != value.RawDigest || !bytes.Equal(stored.Original, value.Original) ||
		stored.ID != value.ID || stored.Type != value.Type || stored.Document != value.Document ||
		stored.Sequence != value.Sequence || stored.Release != value.Release ||
		stored.PublicationEpoch != value.PublicationEpoch || !stored.OccurredAt.Equal(value.OccurredAt) {
		return errors.New("SEARCH_ORIGINAL_EVENT_INVALID")
	}
	return nil
}
