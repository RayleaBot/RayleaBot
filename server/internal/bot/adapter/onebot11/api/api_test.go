package api

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type fakeCaller struct {
	data     map[string]map[string]any
	anyData  map[string]any
	requests []apiRequest
}

type apiRequest struct {
	action    string
	params    map[string]any
	transport string
}

func (c *fakeCaller) CallAPI(_ context.Context, action string, params map[string]any) (map[string]any, error) {
	c.requests = append(c.requests, apiRequest{action: action, params: params})
	if data, ok := c.data[action]; ok {
		return data, nil
	}
	return nil, errors.New("missing response")
}

func (c *fakeCaller) CallAPIAny(_ context.Context, action string, params map[string]any) (any, error) {
	c.requests = append(c.requests, apiRequest{action: action, params: params})
	if data, ok := c.anyData[action]; ok {
		return data, nil
	}
	return nil, errors.New("missing response")
}

func (c *fakeCaller) CallAPIOnTransport(_ context.Context, transport string, action string, params map[string]any) (map[string]any, error) {
	c.requests = append(c.requests, apiRequest{action: action, params: params, transport: transport})
	if data, ok := c.data[action]; ok {
		return data, nil
	}
	return nil, errors.New("missing response")
}

func (c *fakeCaller) TargetValue(targetID string) any {
	return "target:" + targetID
}

func (c *fakeCaller) Errorf(code, message string, err error) error {
	return errors.New(code + ": " + message)
}

func TestClientGetGroupMemberInfoBuildsNoCacheRequest(t *testing.T) {
	caller := &fakeCaller{
		data: map[string]map[string]any{
			"get_group_member_info": {
				"role":     "admin",
				"nickname": "Nick",
				"card":     "Card",
				"title":    "Title",
			},
		},
	}

	info, err := NewClient(caller).GetGroupMemberInfo(context.Background(), "100", "200")
	if err != nil {
		t.Fatalf("GetGroupMemberInfo failed: %v", err)
	}
	if info.Role != "admin" || info.Nickname != "Nick" || info.Card != "Card" || info.Title != "Title" {
		t.Fatalf("unexpected member info: %#v", info)
	}
	if len(caller.requests) != 1 {
		t.Fatalf("expected one API request, got %d", len(caller.requests))
	}
	params := caller.requests[0].params
	if params["group_id"] != "target:100" || params["user_id"] != "target:200" || params["no_cache"] != true {
		t.Fatalf("unexpected params: %#v", params)
	}
}

func TestClientListGroupsSortsSelectableTargets(t *testing.T) {
	caller := &fakeCaller{
		anyData: map[string]any{
			"get_group_list": []any{
				map[string]any{"group_id": float64(2), "group_name": "Beta"},
				map[string]any{"group_id": float64(1), "group_name": "Alpha"},
				map[string]any{"group_id": float64(3), "group_name": "Alpha"},
			},
		},
	}

	groups, err := NewClient(caller).ListGroups(context.Background())
	if err != nil {
		t.Fatalf("ListGroups failed: %v", err)
	}
	want := []GroupTarget{
		{ID: "1", Name: "Alpha"},
		{ID: "3", Name: "Alpha"},
		{ID: "2", Name: "Beta"},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("unexpected groups: got %#v want %#v", groups, want)
	}
}

func TestClientListGroupsAcceptsWrappedGroupPayload(t *testing.T) {
	caller := &fakeCaller{
		anyData: map[string]any{
			"get_group_list": map[string]any{
				"groups": []any{
					map[string]any{"group_id": float64(2), "group_name": "Beta"},
					map[string]any{"group_id": float64(1), "group_name": "Alpha"},
				},
			},
		},
	}

	groups, err := NewClient(caller).ListGroups(context.Background())
	if err != nil {
		t.Fatalf("ListGroups failed: %v", err)
	}
	want := []GroupTarget{
		{ID: "1", Name: "Alpha"},
		{ID: "2", Name: "Beta"},
	}
	if !reflect.DeepEqual(groups, want) {
		t.Fatalf("unexpected groups: got %#v want %#v", groups, want)
	}
}

func TestClientListFriendsAcceptsWrappedFriendPayload(t *testing.T) {
	caller := &fakeCaller{
		anyData: map[string]any{
			"get_friend_list": map[string]any{
				"data": map[string]any{
					"friends": []any{
						map[string]any{"user_id": float64(2), "nickname": "Beta"},
						map[string]any{"user_id": float64(1), "nickname": "Alpha"},
					},
				},
			},
		},
	}

	friends, err := NewClient(caller).ListFriends(context.Background())
	if err != nil {
		t.Fatalf("ListFriends failed: %v", err)
	}
	want := []FriendTarget{
		{ID: "1", Nickname: "Alpha"},
		{ID: "2", Nickname: "Beta"},
	}
	if !reflect.DeepEqual(friends, want) {
		t.Fatalf("unexpected friends: got %#v want %#v", friends, want)
	}
}

func TestResolveTargetNameUsesResolverByTargetType(t *testing.T) {
	resolver := fakeTargetResolver{
		groups:   map[string]string{"g1": "Group One"},
		privates: map[string]string{"u1": "User One"},
	}

	if got := ResolveTargetName(context.Background(), "group", "g1", resolver); got != "Group One" {
		t.Fatalf("unexpected group target name: %q", got)
	}
	if got := ResolveTargetName(context.Background(), "private", "u1", resolver); got != "User One" {
		t.Fatalf("unexpected private target name: %q", got)
	}
	if got := ResolveTargetName(context.Background(), "channel", "c1", resolver); got != "" {
		t.Fatalf("unexpected unsupported target name: %q", got)
	}
}

type fakeTargetResolver struct {
	groups   map[string]string
	privates map[string]string
}

func (r fakeTargetResolver) ResolveGroupName(_ context.Context, targetID string) string {
	return r.groups[targetID]
}

func (r fakeTargetResolver) ResolvePrivateName(_ context.Context, targetID string) string {
	return r.privates[targetID]
}
