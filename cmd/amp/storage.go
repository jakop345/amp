package main

import (
	"errors"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/appcelerator/amp/api/client"
	"github.com/appcelerator/amp/api/rpc/storage"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

// StorageCmd is the main command for attaching storage subcommands.
var StorageCmd = &cobra.Command{
	Use:   "kv",
	Short: "Storage operations",
	Long:  `Manage storage-related operations.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		return AMP.Connect()
	},
}

var (
	// storagePutCmd represents the creation of storage key-value pair
	storagePutCmd = &cobra.Command{
		Use:   "put [key] [val]",
		Short: "Assign specified value with specified key",
		Long:  `Assign specified value with specified key`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return storagePut(AMP, cmd, args)
		},
	}
	// storageGetCmd represents the retrieval of storage value based on key
	storageGetCmd = &cobra.Command{
		Use:   "get [key]",
		Short: "Retrieve a storage object",
		Long:  `Retrieve a storage object`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return storageGet(AMP, cmd, args)
		},
	}
	// storageDeleteCmd represents the deletion of storage value based on key
	storageDeleteCmd = &cobra.Command{
		Use:     "del [key]",
		Short:   "Delete a storage object(alias: rm)",
		Long:    `Delete a storage object(alias: rm)`,
		Aliases: []string{"rm"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return storageDelete(AMP, cmd, args)
		},
	}
	// storageListCmd represents the list of storage key-value pair
	storageListCmd = &cobra.Command{
		Use:   "ls",
		Short: "List all storage objects",
		Long:  `List all storage objects`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return storageList(AMP, cmd, args)
		},
	}
)

func init() {
	RootCmd.PersistentFlags().BoolP("help", "h", false, "Print usage")
	RootCmd.AddCommand(StorageCmd)
	StorageCmd.AddCommand(storagePutCmd)
	StorageCmd.AddCommand(storageGetCmd)
	StorageCmd.AddCommand(storageDeleteCmd)
	StorageCmd.AddCommand(storageListCmd)
}

// storagePut validates the input command line arguments and creates storage key-value pair
// by invoking the corresponding rpc/storage method
func storagePut(amp *client.AMP, cmd *cobra.Command, args []string) (err error) {
	if len(args) < 2 {
		return errors.New("Must specify storage key/value - either or both of them are missing")
	} else if len(args) > 2 {
		return errors.New("Too many arguments - check again!")
	}

	k := args[0]
	if k == "" {
		return errors.New("Must specify storage key")
	}

	v := args[1]
	if v == "" {
		return errors.New("Must specify storage value")
	}

	request := &storage.PutStorage{Key: k, Val: v}

	client := storage.NewStorageClient(amp.Conn)
	reply, err := client.Put(context.Background(), request)
	if err != nil {
		fmt.Println("Key not found: " + k)
		return nil
	}
	fmt.Println(reply.Val)
	return nil
}

// storageGet validates the input command line arguments and retrieves storage key-value pair
//by invoking the corresponding rpc/storage method
func storageGet(amp *client.AMP, cmd *cobra.Command, args []string) (err error) {
	if len(args) > 1 {
		return errors.New("Too many arguments - check again!")
	} else if len(args) == 0 {
		return errors.New("Must specify storage key")
	}
	k := args[0]
	if k == "" {
		return errors.New("Must specify storage key")
	}

	request := &storage.GetStorage{Key: k}

	client := storage.NewStorageClient(amp.Conn)
	reply, err := client.Get(context.Background(), request)
	if err != nil {
		fmt.Println("Key not found: " + k)
		return nil
	}
	fmt.Println(reply.Val)
	return nil
}

// storageDelete validates the input command line arguments and deletes storage key-value pair
// by invoking the corresponding rpc/storage method
func storageDelete(amp *client.AMP, cmd *cobra.Command, args []string) (err error) {
	if len(args) > 1 {
		return errors.New("Too many arguments - check again!")
	} else if len(args) == 0 {
		return errors.New("Must specify storage key")
	}
	k := args[0]
	if k == "" {
		return errors.New("Must specify storage key")
	}

	request := &storage.DeleteStorage{Key: k}

	client := storage.NewStorageClient(amp.Conn)
	reply, err := client.Delete(context.Background(), request)
	if err != nil {
		fmt.Println("Key not found: " + k)
		return nil
	}
	fmt.Println(reply.Val)
	return nil
}

// storageUpdate validates the input command line arguments and updates storage key-value pair
// by invoking the corresponding rpc/storage method
// func storageUpdate(amp *client.AMP, cmd *cobra.Command, args []string) (err error) {
// 	if len(args) < 2 {
// 		return errors.New("Must specify storage key/value - either or both of them are missing")
// 	} else if len(args) > 2 {
// 		return errors.New("Too many arguments - check again!")
// 	}
//
// 	k := args[0]
// 	if k == "" {
// 		return errors.New("Must specify storage key")
// 	}
//
// 	v := args[1]
// 	if v == "" {
// 		return errors.New("Must specify storage value")
// 	}
//
// 	request := &storage.UpdateStorage{Key: k, Val: v}
//
// 	client := storage.NewStorageClient(amp.Conn)
// 	reply, err := client.Update(context.Background(), request)
// 	if err != nil {
// 		return err
// 	}
// 	fmt.Println(reply.Val)
// 	return nil
// }

// storageList validates the input command line arguments and lists all the storage
// key-value pairs by invoking the corresponding rpc/storage method
func storageList(amp *client.AMP, cmd *cobra.Command, args []string) (err error) {
	if len(args) > 0 {
		return errors.New("Too many arguments - check again!")
	}
	request := &storage.ListStorage{}
	client := storage.NewStorageClient(amp.Conn)
	reply, err := client.List(context.Background(), request)
	if err != nil {
		return err
	}
	if reply == nil || len(reply.List) == 0 {
		fmt.Println("No storage object is available")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', 0)
	fmt.Fprintln(w, "KEY\tVALUE\t")
	fmt.Fprintln(w, "---\t-----\t")
	for _, info := range reply.List {
		fmt.Fprintf(w, "%s\t%s\t\n", info.Key, info.Val)
	}
	w.Flush()
	return nil
}
