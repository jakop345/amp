package function

import (
	"fmt"
	"github.com/appcelerator/amp/config"
	"github.com/appcelerator/amp/data/storage"
	"github.com/appcelerator/amp/pkg/nats-streaming"
	"github.com/docker/docker/pkg/stringid"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"log"
	"path"
	"strings"
)

// Server is used to implement function.FunctionServer
type Server struct {
	Store         storage.Interface
	NatsStreaming ns.NatsStreaming
}

// Create implements function.Server
func (s *Server) Create(ctx context.Context, in *CreateRequest) (*CreateReply, error) {
	// Check if the function already exists
	reply, err := s.List(ctx, &ListRequest{})
	if err != nil {
		return nil, err
	}
	for _, fn := range reply.Functions {
		if strings.EqualFold(fn.Name, in.Function.Name) {
			return nil, fmt.Errorf("Function already exists: %s", in.Function.Name)
		}
	}

	// Validate the function
	fn := in.Function
	if fn.Image == "" {
		return nil, errors.New("Image is mandatory")
	}

	// Store the function
	fn.Id = stringid.GenerateNonCryptoID()
	if err := s.Store.Create(ctx, path.Join(amp.EtcdFunctionRootKey, fn.Id), fn, nil, 0); err != nil {
		return nil, err
	}
	log.Println("Created function: ", fn.String())
	return &CreateReply{Function: fn}, nil
}

// List implements function.Server
func (s *Server) List(ctx context.Context, in *ListRequest) (*ListReply, error) {
	var functions []proto.Message
	if err := s.Store.List(ctx, amp.EtcdFunctionRootKey, storage.Everything, &FunctionEntry{}, &functions); err != nil {
		return nil, err
	}
	reply := &ListReply{}
	for _, function := range functions {
		reply.Functions = append(reply.Functions, function.(*FunctionEntry))
	}
	return reply, nil
}

// Delete implements function.Server
func (s *Server) Delete(ctx context.Context, in *DeleteRequest) (*DeleteReply, error) {
	function := &FunctionEntry{}
	if err := s.Store.Get(ctx, path.Join(amp.EtcdFunctionRootKey, in.Id), function, false); err != nil {
		return nil, fmt.Errorf("Function not found: %s", in.Id)
	}

	if err := s.Store.Delete(ctx, path.Join(amp.EtcdFunctionRootKey, in.Id), false, nil); err != nil {
		return nil, err
	}

	return &DeleteReply{Function: function}, nil
}
