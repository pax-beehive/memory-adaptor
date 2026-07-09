package zep

import (
	"context"
	"errors"
	"net/http"
	"testing"

	zepgo "github.com/getzep/zep-go/v3"
	"github.com/getzep/zep-go/v3/core"
	"github.com/getzep/zep-go/v3/option"
)

type fakeUserClient struct {
	getErr     error
	addErr     error
	getCalls   int
	addCalls   int
	addRequest *zepgo.CreateUserRequest
}

func (c *fakeUserClient) Get(context.Context, string, ...option.RequestOption) (*zepgo.User, error) {
	c.getCalls++
	if c.getErr != nil {
		return nil, c.getErr
	}
	return &zepgo.User{}, nil
}

func (c *fakeUserClient) Add(_ context.Context, request *zepgo.CreateUserRequest, _ ...option.RequestOption) (*zepgo.User, error) {
	c.addCalls++
	c.addRequest = request
	if c.addErr != nil {
		return nil, c.addErr
	}
	return &zepgo.User{}, nil
}

func TestEnsureUserReturnsExistingUser(t *testing.T) {
	t.Parallel()

	client := &fakeUserClient{}
	result, err := ensureUserWithClient(context.Background(), "toddzheng", client)
	if err != nil {
		t.Fatal(err)
	}
	if result.UserID != "toddzheng" || result.Created {
		t.Fatalf("unexpected ensure result: %#v", result)
	}
	if client.getCalls != 1 || client.addCalls != 0 {
		t.Fatalf("unexpected client calls: get=%d add=%d", client.getCalls, client.addCalls)
	}
}

func TestEnsureUserCreatesMissingUser(t *testing.T) {
	t.Parallel()

	client := &fakeUserClient{getErr: apiStatusError(http.StatusNotFound)}
	result, err := ensureUserWithClient(context.Background(), "toddzheng", client)
	if err != nil {
		t.Fatal(err)
	}
	if result.UserID != "toddzheng" || !result.Created {
		t.Fatalf("unexpected ensure result: %#v", result)
	}
	if client.addRequest == nil || client.addRequest.UserID != "toddzheng" {
		t.Fatalf("unexpected add request: %#v", client.addRequest)
	}
}

func TestEnsureUserTreatsCreateConflictAsExisting(t *testing.T) {
	t.Parallel()

	client := &fakeUserClient{
		getErr: apiStatusError(http.StatusNotFound),
		addErr: apiStatusError(http.StatusConflict),
	}
	result, err := ensureUserWithClient(context.Background(), "toddzheng", client)
	if err != nil {
		t.Fatal(err)
	}
	if result.UserID != "toddzheng" || result.Created {
		t.Fatalf("unexpected ensure result: %#v", result)
	}
}

func TestEnsureUserReturnsNonNotFoundGetErrors(t *testing.T) {
	t.Parallel()

	client := &fakeUserClient{getErr: errors.New("offline")}
	if _, err := ensureUserWithClient(context.Background(), "toddzheng", client); err == nil {
		t.Fatal("expected get error")
	}
}

func apiStatusError(status int) error {
	return core.NewAPIError(status, nil, errors.New(http.StatusText(status)))
}
