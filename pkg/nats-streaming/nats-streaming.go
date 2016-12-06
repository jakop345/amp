package ns

import (
	"fmt"
	"github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/nats"
	"log"
	"time"
)

// NATS-Streaming wrapper
type NatsStreaming struct {
	client stan.Conn
}

// Connect to NATS-Streaming
func (ns *NatsStreaming) Connect(url string, clusterId string, clientId string, timeout time.Duration) error {
	log.Printf("Connecting to NATS-Streaming, url: %s, clusterId: %s, clientId: %s, timeout: %s\n", url, clusterId, clientId, timeout)
	nc, err := nats.Connect(url, nats.Timeout(timeout))
	if err != nil {
		return fmt.Errorf("Unable to connect to NATS-Streaming: %v", err)
	}
	ns.client, err = stan.Connect(clusterId, clientId, stan.NatsConn(nc), stan.ConnectWait(timeout))
	if err != nil {
		return fmt.Errorf("Unable to connect to NATS-Streaming: %v", err)
	}
	log.Println("Connected to NATS-Streaming successfuly")
	return nil
}

func (ns *NatsStreaming) GetClient() stan.Conn {
	return ns.client
}

func (ns *NatsStreaming) Close() error {
	return ns.client.Close()
}
