package staff

import "context"

type ctxKey string

const staffKey ctxKey = "staff"

// WithContext 把当前操作员放进请求上下文（中间件校验通过后调用）。
func WithContext(ctx context.Context, st Staff) context.Context {
	return context.WithValue(ctx, staffKey, st)
}

// FromContext 取出当前操作员。
func FromContext(ctx context.Context) (Staff, bool) {
	st, ok := ctx.Value(staffKey).(Staff)
	return st, ok
}
