package storage

import (
	"fmt"
	"path"

	"github.com/appcelerator/amp/data/storage"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
)

const storageRootKey = "storage"

// Server is used to implement storage service
type Server struct {
	Store storage.Interface
}

// Put implements Server API for Put Storage service
func (s *Server) Put(ctx context.Context, in *PutStorage) (*StorageResponse, error) {
	//verify if key-value pair does not already exist
	getStorage := s.getStorageByKey(ctx, in.Key)
	if getStorage.Val != "" {
		return nil, fmt.Errorf("Storage key %s already exists", in.Key)
	}

	storage := &StorageResponse{
		Key: in.Key,
		Val: in.Val,
	}
	//save storage data in ETCD
	if err := s.Store.Put(ctx, path.Join(storageRootKey, in.Key), storage, 0); err != nil {
		return nil, err
	}

	return storage, nil
}

// Get implements Server API for Get Storage service
func (s *Server) Get(ctx context.Context, in *GetStorage) (*StorageResponse, error) {
	var response *StorageResponse
	response = s.getStorageByKey(ctx, in.Key)
	if response.Val == "" {
		return nil, fmt.Errorf("Storage key %s does not exist", in.Key)
	}
	return response, nil
}

// Retrieve storage key-value pair by key
func (s *Server) getStorageByKey(ctx context.Context, key string) *StorageResponse {
	storage := &StorageResponse{}
	//get storage data from ETCD
	s.Store.Get(ctx, path.Join(storageRootKey, key), storage, true)
	return storage
}

// Delete implements Server API for Delete Storage service
func (s *Server) Delete(ctx context.Context, in *DeleteStorage) (*StorageResponse, error) {
	var response *StorageResponse
	response = s.getStorageByKey(ctx, in.Key)
	if response.Val == "" {
		return nil, fmt.Errorf("Storage key %s does not exist", in.Key)
	}
	//delete storage data in ETCD
	s.Store.Delete(ctx, path.Join(storageRootKey, in.Key), true, nil)
	return response, nil
}

// Update implements Server API for Update Storage service
// func (s *Server) Update(ctx context.Context, in *UpdateStorage) (*StorageResponse, error) {
// 	//verify if key-value pair does not already exist
// 	getStorage := s.getStorageByKey(ctx, in.Key)
// 	if getStorage.Val == "" {
// 		return nil, fmt.Errorf("Storage key %s does not exist", in.Key)
// 	}
// 	//set value for storageResponse
// 	storage := &StorageResponse{
// 		Key: in.Key,
// 		Val: in.Val,
// 	}
// 	//update storage data in ETCD
// 	s.Store.Update(ctx, path.Join(storageRootKey, in.Key), storage, 0)
//
// 	return storage, nil
// }

// List implements Server API for List Storage service
func (s *Server) List(ctx context.Context, in *ListStorage) (*ListResponse, error) {
	var idList []proto.Message
	err := s.Store.List(ctx, storageRootKey, storage.Everything, &StorageKey{}, &idList)
	if err != nil {
		return nil, err
	}
	listInfo := []*StorageInfo{}
	for _, k := range idList {
		obj, _ := k.(*StorageKey)
		info := s.getStorageInfo(ctx, obj.Key)
		fmt.Println("info ::", info)
		listInfo = append(listInfo, s.getStorageInfo(ctx, obj.Key))
	}
	//set value for ListResponse
	response := &ListResponse{
		List: listInfo,
	}
	return response, nil
}

// return information to be displayed in storage ls
func (s *Server) getStorageInfo(ctx context.Context, key string) *StorageInfo {
	info := StorageInfo{}
	storage := StorageResponse{}
	err := s.Store.Get(ctx, path.Join(storageRootKey, key), &storage, true)
	if err == nil {
		info.Key = key
		info.Val = storage.Val
	}
	return &info
}
