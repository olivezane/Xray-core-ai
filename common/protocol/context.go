package protocol

import (
	"context"
)

type key int

const (
	requestKey key = iota
)

func ContextWithRequestHeader(ctx context.Context, request *RequestHeader) context.Context {
	return context.WithValue(ctx, requestKey, request)
}

func RequestHeaderFromContext(ctx context.Context) *RequestHeader {
	request := ctx.Value(requestKey)
	if request == nil {
		return nil
	}
	//nolint:forcetypeassert // the context is only ever given a *RequestHeader
	return request.(*RequestHeader)
}
