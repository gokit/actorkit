package natstreaming_test

import (
	"context"
	"testing"

	"github.com/gokit/actorkit/internal"
	"github.com/gokit/actorkit/pubsubs/internal/encoders"
	"github.com/stretchr/testify/assert"

	streaming "github.com/gokit/actorkit/pubsubs/natstreaming"

	"github.com/gokit/actorkit"
	"github.com/gokit/actorkit/pubsubs"
	"github.com/gokit/actorkit/pubsubs/internal/benches"
)

func TestNATS(t *testing.T) {
	natspub, err := streaming.NewPublisherSubscriberFactory(context.Background(), streaming.Config{
		URL:         "localhost:4222",
		ClusterID:   "cluster_server",
		ProjectID:   "wireco",
		Log:         &internal.TLog{},
		Marshaler:   encoders.NoAddressMarshaler{},
		Unmarshaler: encoders.NoAddressUnmarshaler{},
	})

	assert.NoError(t, err)
	assert.NotNil(t, natspub)

	if err != nil {
		return
	}

	defer natspub.Close()

	factory := streaming.PubSubFactory(func(factory *streaming.PublisherSubscriberFactory, topic string) (pubsubs.Publisher, error) {
		return factory.Publisher(topic)
	}, func(factory *streaming.PublisherSubscriberFactory, topic string, id string, receiver pubsubs.Receiver) (actorkit.Subscription, error) {
		return factory.Subscribe(topic, id, receiver, nil)
	})(natspub)

	benches.PubSubFactoryTestSuite(t, factory)
}
