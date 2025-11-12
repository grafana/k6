package redis

import "context"

type ACLCmdable interface {
	ACLDryRun(ctx context.Context, username string, command ...interface{}) *StringCmd
	ACLLog(ctx context.Context, count int64) *ACLLogCmd
	ACLLogReset(ctx context.Context) *StatusCmd
}

func (c cmdable) ACLDryRun(ctx context.Context, username string, command ...interface{}) *StringCmd {
	args := make([]interface{}, 0, 3+len(command))
	args = append(args, "acl", "dryrun", username)
	args = append(args, command...)
	cmd := NewStringCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ACLLog(ctx context.Context, count int64) *ACLLogCmd {
	args := make([]interface{}, 0, 3)
	args = append(args, "acl", "log")
	if count > 0 {
		args = append(args, count)
	}
	cmd := NewACLLogCmd(ctx, args...)
	_ = c(ctx, cmd)
	return cmd
}

func (c cmdable) ACLLogReset(ctx context.Context) *StatusCmd {
	cmd := NewStatusCmd(ctx, "acl", "log", "reset")
	_ = c(ctx, cmd)
	return cmd
}
