package core

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/appcelerator/amp/config"
	"github.com/appcelerator/amp/data/elasticsearch"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/nats-io/go-nats-streaming"
	"github.com/nats-io/nats"
	"golang.org/x/net/context"
	"log"
	"math/rand"
	"strconv"
)

const (
	defaultTimeOut = 30 * time.Second
	natsClientID   = "amp-agent"
)

//Agent data
type Agent struct {
	dockerClient        *client.Client
	containers          map[string]*ContainerData
	eventStreamReading  bool
	lastUpdate          time.Time
	natsClient          stan.Conn
	elasticsearchClient elasticsearch.Elasticsearch
}

//ContainerData data
type ContainerData struct {
	name          string
	labels        map[string]string
	state         string
	health        string
	logsStream    io.ReadCloser
	logsReadError bool
}

var agent Agent

//AgentInit Connect to docker engine, get initial containers list and start the agent
func AgentInit(version string, build string) error {
	agent.trapSignal()
	conf.init(version, build)

	// Connection to Elasticsearch
	log.Printf("Connecting to elasticsearch at %s\n", conf.elasticsearchURL)
	if err := agent.elasticsearchClient.Connect(conf.elasticsearchURL, defaultTimeOut); err != nil {
		return fmt.Errorf("amplifer is unable to connect to elasticsearch on: %s\n%v", conf.elasticsearchURL, err)
	}
	log.Printf("Connected to elasticsearch at %s\n", conf.elasticsearchURL)

	// Connection to NATS
	log.Printf("Connecting to NATS-Streaming at %s\n", conf.natsURL)
	nc, err := nats.Connect(conf.natsURL, nats.Timeout(defaultTimeOut))
	if err != nil {
		fmt.Errorf("amp-agent is unable to connect to NATS on: %s\n%v", conf.natsURL, err)
	}
	agent.natsClient, err = stan.Connect(amp.NatsClusterID, natsClientID+strconv.Itoa(rand.Int()), stan.NatsConn(nc), stan.ConnectWait(defaultTimeOut))
	if err != nil {
		return fmt.Errorf("amp-agent is unable to connect to NATS-Streaming on: %s\n%v", conf.natsURL, err)
	}
	log.Printf("Connected to NATS-Streaming at %s\n", conf.natsURL)

	// Connection to Docker
	defaultHeaders := map[string]string{"User-Agent": "engine-api-cli-1.0"}
	cli, err := client.NewClient(conf.dockerEngine, "v1.24", nil, defaultHeaders)
	if err != nil {
		agent.natsClient.Close()
		return err
	}
	agent.dockerClient = cli
	fmt.Println("Connected to Docker-engine")

	fmt.Println("Extracting containers list...")
	agent.containers = make(map[string]*ContainerData)
	ContainerListOptions := types.ContainerListOptions{All: true}
	containers, err := agent.dockerClient.ContainerList(context.Background(), ContainerListOptions)
	if err != nil {
		agent.natsClient.Close()
		return err
	}
	for _, cont := range containers {
		agent.addContainer(cont.ID)
	}
	agent.lastUpdate = time.Now()
	fmt.Println("done")
	agent.start()
	return nil
}

//Main agent loop, verify if events and logs stream are started if not start them
func (agt *Agent) start() {
	initAPI()
	for {
		updateLogsStream()
		updateEventsStream()
		time.Sleep(time.Duration(conf.period) * time.Second)
	}
}

//Update containers list concidering event action and event containerId
func (agt *Agent) updateContainerMap(action string, containerID string) {
	if action == "start" {
		agt.addContainer(containerID)
	} else if action == "destroy" || action == "die" || action == "kill" || action == "stop" {
		go func() {
			time.Sleep(5 * time.Second)
			agt.removeContainer(containerID)
		}()
	}
}

//add a container to the main container map and retrieve some container information
func (agt *Agent) addContainer(ID string) {
	_, ok := agt.containers[ID]
	if !ok {
		inspect, err := agt.dockerClient.ContainerInspect(context.Background(), ID)
		if err == nil {
			data := ContainerData{
				name:          inspect.Name,
				labels:        inspect.Config.Labels,
				state:         inspect.State.Status,
				health:        "",
				logsStream:    nil,
				logsReadError: false,
			}
			if inspect.State.Health != nil {
				data.health = inspect.State.Health.Status
			}
			if data.labels["io.amp.stack.name"] == "" {
				fmt.Printf("add infrastructure container  %s\n", data.name)
			} else {
				fmt.Printf("add user container %s, stack=%s service=%s\n", data.name, data.labels["io.amp.stack.name"], data.labels["com.docker.swarm.service.name"])
			}
			agt.containers[ID] = &data
		} else {
			fmt.Printf("Container inspect error: %v\n", err)
		}
	}
}

//Suppress a container from the main container map
func (agt *Agent) removeContainer(ID string) {
	data, ok := agent.containers[ID]
	if ok {
		fmt.Println("remove container", data.name)
		delete(agt.containers, ID)
	}
}

func (agt *Agent) updateContainer(ID string) {
	data, ok := agt.containers[ID]
	if ok {
		inspect, err := agt.dockerClient.ContainerInspect(context.Background(), ID)
		if err == nil {
			data.labels = inspect.Config.Labels
			data.state = inspect.State.Status
			data.health = ""
			if inspect.State.Health != nil {
				data.health = inspect.State.Health.Status
			}
			fmt.Println("update container", data.name)
		} else {
			fmt.Printf("Container %s inspect error: %v\n", data.name, err)
		}
	}
}

// Launch a routine to catch SIGTERM Signal
func (agt *Agent) trapSignal() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	signal.Notify(ch, syscall.SIGTERM)
	go func() {
		<-ch
		fmt.Println("\namp-agent received SIGTERM signal")
		closeLogsStreams()
		agent.natsClient.Close()
		os.Exit(1)
	}()
}
