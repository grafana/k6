package redis

import (
	"context"
	"fmt"
	"strings"
)

type GearsCmdable interface {
	TFunctionLoad(ctx context.Context, lib string) *StatusCmd
	TFunctionLoadArgs(ctx context.Context, lib string, options *TFunctionLoadOptions) *StatusCmd
	TFunctionDelete(ctx context.Context, libName string) *StatusCmd
	TFunctionList(ctx context.Context) *MapStringInterfaceSliceCmd
	TFunctionListArgs(ctx context.Context, options *TFunctionListOptions) *MapStringInterfaceSliceCmd
	TFCall(ctx context.Context, libName string, funcName string, numKeys int) *Cmd
	TFCallArgs(ctx context.Context, libName string, funcName string, numKeys int, options *TFCallOptions) *Cmd
	TFCallASYNC(ctx context.Context, libName string, funcName string, numKeys int) *Cmd
	TFCallASYNCArgs(ctx context.Context, libName string, funcName string, numKeys int, options *TFCallOptions) *Cmd
}

type TFunctionLoadOptions struct {
	Replace bool
	Config  string
}

type TFunctionListOptions struct {
	Withcode bool
	Verbose  int
	Library  string
}

type TFCallOptions struct {
	Keys      []string
	Arguments []string
}

// TFunctionLoad - load a new JavaScript library into Redis.
// For more information - https://redis.io/commands/tfunction-load/
func (c cmdable) TFunctionLoad(ctx context.Context, lib string) *StatusCmd {
	args := []interface{}{"TFUNCTION", "LOAD", lib}
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) TFunctionLoadArgs(ctx context.Context, lib string, options *TFunctionLoadOptions) *StatusCmd {
	args := []interface{}{"TFUNCTION", "LOAD"}
	if options != nil {
		if options.Replace {
			args = append(args, "REPLACE")
		}
		if options.Config != "" {
			args = append(args, "CONFIG", options.Config)
		}
	}
	args = append(args, lib)
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// TFunctionDelete - delete a JavaScript library from Redis.
// For more information - https://redis.io/commands/tfunction-delete/
func (c cmdable) TFunctionDelete(ctx context.Context, libName string) *StatusCmd {
	args := []interface{}{"TFUNCTION", "DELETE", libName}
	cmd := NewStatusCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// TFunctionList - list the functions with additional information about each function.
// For more information - https://redis.io/commands/tfunction-list/
func (c cmdable) TFunctionList(ctx context.Context) *MapStringInterfaceSliceCmd {
	args := []interface{}{"TFUNCTION", "LIST"}
	cmd := NewMapStringInterfaceSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) TFunctionListArgs(ctx context.Context, options *TFunctionListOptions) *MapStringInterfaceSliceCmd {
	args := []interface{}{"TFUNCTION", "LIST"}
	if options != nil {
		if options.Withcode {
			args = append(args, "WITHCODE")
		}
		if options.Verbose != 0 {
			v := strings.Repeat("v", options.Verbose)
			args = append(args, v)
		}
		if options.Library != "" {
			args = append(args, "LIBRARY", options.Library)
		}
	}
	cmd := NewMapStringInterfaceSliceCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// TFCall - invoke a function.
// For more information - https://redis.io/commands/tfcall/
func (c cmdable) TFCall(ctx context.Context, libName string, funcName string, numKeys int) *Cmd {
	lf := libName + "." + funcName
	args := []interface{}{"TFCALL", lf, numKeys}
	cmd := NewCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) TFCallArgs(ctx context.Context, libName string, funcName string, numKeys int, options *TFCallOptions) *Cmd {
	lf := libName + "." + funcName
	args := []interface{}{"TFCALL", lf, numKeys}
	if options != nil {
		for _, key := range options.Keys {
			args = append(args, key)
		}
		for _, key := range options.Arguments {
			args = append(args, key)
		}
	}
	cmd := NewCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

// TFCallASYNC - invoke an asynchronous JavaScript function (coroutine).
// For more information - https://redis.io/commands/TFCallASYNC/
func (c cmdable) TFCallASYNC(ctx context.Context, libName string, funcName string, numKeys int) *Cmd {
	lf := fmt.Sprintf("%s.%s", libName, funcName)
	args := []interface{}{"TFCALLASYNC", lf, numKeys}
	cmd := NewCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) TFCallASYNCArgs(ctx context.Context, libName string, funcName string, numKeys int, options *TFCallOptions) *Cmd {
	lf := fmt.Sprintf("%s.%s", libName, funcName)
	args := []interface{}{"TFCALLASYNC", lf, numKeys}
	if options != nil {
		for _, key := range options.Keys {
			args = append(args, key)
		}
		for _, key := range options.Arguments {
			args = append(args, key)
		}
	}
	cmd := NewCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}
