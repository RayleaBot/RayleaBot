package api

import "context"

type Caller interface {
	CallAPI(context.Context, string, map[string]any) (map[string]any, error)
	CallAPIAny(context.Context, string, map[string]any) (any, error)
	CallAPIOnTransport(context.Context, string, string, map[string]any) (map[string]any, error)
	TargetValue(string) any
	Errorf(string, string, error) error
}

type Client struct {
	caller Caller
}

func NewClient(caller Caller) Client {
	return Client{caller: caller}
}
