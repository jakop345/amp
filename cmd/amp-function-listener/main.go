package main

import (
	"fmt"
	"github.com/appcelerator/amp/api/rpc/function"
	"github.com/appcelerator/amp/config"
	"github.com/appcelerator/amp/data/storage"
	"github.com/appcelerator/amp/data/storage/etcd"
	"github.com/appcelerator/amp/pkg/nats-streaming"
	"github.com/docker/docker/pkg/stringid"
	"github.com/gogo/protobuf/proto"
	"github.com/julienschmidt/httprouter"
	"github.com/nats-io/go-nats-streaming"
	"golang.org/x/net/context"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

// ## `amp-function-listener`
// This service role is to:
// - listen to HTTP events:
//    - Parse the HTTP body (if any) and use it as an input for the function
//    - Publish function call to NATS "function call" topic
//    - Wait on a channel with a timeout of one minute
//
// - listen to NATS for function returns on the "returnTo" topic. There is one "returnTo" topic per `amp-function-listener` used by workers to submit function return.
//   - Store the function return in a map
//   - Unblock the HTTP handler
//   - Get the output of the function (if any) and write it as the HTTP body
//   - In case of timeout, return an error

// build vars
var (
	// Version is set with a linker flag (see Makefile)
	Version string

	// Build is set with a linker flag (see Makefile)
	Build string

	// natsStreaming Streaming
	natsStreaming ns.NatsStreaming

	// store is the interface used to access the key/value storage backend
	store storage.Interface

	// hostname is used in NATS client identifier and for returnTo
	hostname string

	// holds the locks for function return
	locks = make(map[string](chan bool))

	// holds the function return
	returns = make(map[string](function.FunctionReturn))
)

const (
	listenAddr = ":80"
)

func main() {
	log.Printf("%s (version: %s, build: %s)\n", os.Args[0], Version, Build)

	// Hostname
	var err error
	hostname, err = os.Hostname()
	if err != nil {
		log.Fatalf("Unable to get hostname: %s", err)
	}

	// Storage
	log.Printf("Connecting to etcd at %v", amp.EtcdDefaultEndpoint)
	store = etcd.New([]string{amp.EtcdDefaultEndpoint}, "amp")
	if err := store.Connect(amp.DefaultTimeout); err != nil {
		log.Fatalln("Unable to connect to etcd:", err)
	}
	log.Printf("connected to etcd at %v", strings.Join(store.Endpoints(), ","))

	// NATS Connect
	natsClientId := os.Args[0] + "-" + hostname
	if natsStreaming.Connect(amp.NatsDefaultURL, amp.NatsClusterID, natsClientId, amp.DefaultTimeout) != nil {
		log.Fatal(err)
	}

	// NATS, subscribe to returnTo topic
	returnToTopic := hostname
	_, err = natsStreaming.GetClient().Subscribe(returnToTopic, messageHandler, stan.DeliverAllAvailable())
	if err != nil {
		natsStreaming.Close()
		log.Fatalf("Unable to subscribe to %s topic: %s", returnToTopic, err)
	}
	log.Println("Subscribed to topic:", returnToTopic)

	// HTTP
	router := httprouter.New()
	router.POST("/:id", Index)

	log.Println("Start listening on", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, router))
}

func Index(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
	// Get the function id from the URL
	id := p.ByName("id")

	// Get the function if it exists
	ctx, cancel := context.WithTimeout(context.Background(), amp.DefaultTimeout)
	key := path.Join(amp.EtcdFunctionRootKey, id)
	fn := &function.FunctionEntry{}
	if err := store.Get(ctx, key, fn, false); err != nil {
		log.Println("Function not found:", err)

		// Return the error to the caller
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "Not found", id)
		return
	}
	defer cancel()

	// Read the body parameter if any
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Println("error reading body:", err)

		// Return the error to the caller
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(w, "error reading body:", err)
		return
	}

	// Invoke the function by posting a function call to NATS
	callID := stringid.GenerateNonCryptoID()
	functionCall := function.FunctionCall{
		FunctionID: id,
		CallID:     callID,
		Input:      body,
		ReturnTo:   hostname,
	}

	// Encode the proto object
	encoded, err := proto.Marshal(&functionCall)
	if err != nil {
		log.Println("error marshalling function call:", err)

		// Return the error to the caller
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "error marshalling function call:", err)
		return
	}

	// Publish to NATS
	_, err = natsStreaming.GetClient().PublishAsync(amp.NatsFunctionTopic, encoded, nil)
	if err != nil {
		log.Println("error publishing function call:", err)

		// Return the error to the caller
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(w, "error publishing function call:", err)
		return
	}
	log.Println("function call successfuly submitted:", functionCall.String())

	// Wait for a NATS response for one minute
	timeout := time.After(amp.DefaultTimeout)
	locks[callID] = make(chan bool, 1) // Create the channel
	select {
	case <-locks[callID]:
		// read from the  channel, waiting for answer
		functionReturn := returns[callID] // Get the return object from the map
		delete(returns, callID)           // Delete it from the map
		if _, err := fmt.Fprint(w, string(functionReturn.Output)); err != nil {
			// Write the output
			log.Println("error writing function return:", err)
			return
		}
	case <-timeout:
		// read from the channel has timed out
		// Return an error to the caller
		w.WriteHeader(http.StatusRequestTimeout)
		fmt.Fprintln(w, "function timed out")
		return
	}
}

func messageHandler(msg *stan.Msg) {
	// Parse function return message
	functionReturn := function.FunctionReturn{}
	err := proto.Unmarshal(msg.Data, &functionReturn)
	if err != nil {
		log.Println("error unmarshalling function return:", err)
		return
	}
	log.Println("Received a function return", functionReturn.String())

	// Store the response in the map
	returns[functionReturn.CallID] = functionReturn

	// Unlock the caller
	locks[functionReturn.CallID] <- true
}
