package main

import (
	"errors"
	"fmt"
	"github.com/appcelerator/amp/api/client"
	"github.com/appcelerator/amp/api/rpc/function"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
	"os"
	"text/tabwriter"
)

var (
	functionCmd = &cobra.Command{
		Use:     "function operations",
		Short:   "function operations",
		Long:    `Manage function-related operations.`,
		Aliases: []string{"fn"},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return AMP.Connect()
		},
	}

	createFunctionCmd = &cobra.Command{
		Use:   "create [OPTIONS] NAME",
		Short: "Create a function",
		Long:  `Create a function.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return createFunction(AMP, cmd, args)
		},
	}

	listFunctionCmd = &cobra.Command{
		Use:   "ls",
		Short: "List functions",
		Long:  `List functions.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listFunction(AMP, cmd, args)
		},
	}

	removeFunctionCmd = &cobra.Command{
		Use:   "rm FUNCTION",
		Short: "Remove a function",
		Long:  `Remove a function.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return removeFunction(AMP, cmd, args)
		},
	}
)

func init() {
	createFunctionFlags := createFunctionCmd.Flags()
	createFunctionFlags.StringP("image", "i", "", "The docker image function to execute")

	functionCmd.AddCommand(createFunctionCmd)
	functionCmd.AddCommand(listFunctionCmd)
	functionCmd.AddCommand(removeFunctionCmd)
	RootCmd.AddCommand(functionCmd)
}

func createFunction(amp *client.AMP, cmd *cobra.Command, args []string) error {
	// Validation function name
	if len(args) == 0 {
		return errors.New("function is mandatory")
	}
	name := args[0]
	if name == "" {
		return errors.New("function is mandatory")
	}

	// Validate image
	image := cmd.Flag("image").Value.String()
	if image == "" {
		return errors.New("image is mandatory")
	}

	// Create function
	request := &function.CreateRequest{Function: &function.FunctionEntry{
		Name:  name,
		Image: image,
	}}
	fnClient := function.NewFunctionClient(amp.Conn)
	reply, err := fnClient.Create(context.Background(), request)
	if err != nil {
		return err
	}

	fmt.Println(reply.Function.Id)
	return nil
}

func listFunction(amp *client.AMP, cmd *cobra.Command, args []string) error {
	request := &function.ListRequest{}

	client := function.NewFunctionClient(amp.Conn)
	reply, err := client.List(context.Background(), request)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, tablePadding, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tImage")
	for _, fn := range reply.Functions {
		fmt.Fprintf(w, "%s\t%s\t%s\t\n", fn.Id, fn.Name, fn.Image)
	}
	w.Flush()

	return nil
}

func removeFunction(amp *client.AMP, cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return errors.New("function is mandatory")
	}
	id := args[0]
	if id == "" {
		return errors.New("function is mandatory")
	}

	request := &function.DeleteRequest{Id: id}

	client := function.NewFunctionClient(amp.Conn)
	reply, err := client.Delete(context.Background(), request)
	if err != nil {
		return err
	}
	fmt.Println(reply.Function.Id)
	return nil
}
