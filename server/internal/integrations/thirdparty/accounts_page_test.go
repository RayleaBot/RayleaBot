package thirdparty

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/RayleaBot/RayleaBot/server/internal/pagination"
)

func TestAccountListsReleaseSingleReadConnectionBeforeCredentialReads(t *testing.T) {
	service := newTestService(t)
	for index := range 3 {
		_, err := service.Upsert(t.Context(), UpsertRequest{Platform: PlatformWeibo, AccountID: fmt.Sprintf("account-%d", index), Label: "Label", Cookie: "SUB=fixture", Enabled: true, Profile: AccountProfile{Nickname: fmt.Sprintf("Nickname-%d", index)}})
		if err != nil {
			t.Fatal(err)
		}
	}
	service.read.SetMaxOpenConns(1)
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	page, err := service.ListPage(ctx, pagination.Query{Limit: 1, Cursor: 1, Text: "NICKNAME"})
	if err != nil || page.Total != 3 || len(page.Items) != 1 || page.Items[0].AccountID != "account-1" || !page.Items[0].Configured {
		t.Fatalf("page=%#v err=%v", page, err)
	}
	all, err := service.List(ctx)
	if err != nil || len(all) != 3 {
		t.Fatalf("list=%#v err=%v", all, err)
	}
	enabled, err := service.ListEnabled(ctx, PlatformWeibo)
	if err != nil || len(enabled) != 3 || !enabled[0].Configured {
		t.Fatalf("enabled=%#v err=%v", enabled, err)
	}
	if err := ctx.Err(); err != nil {
		t.Fatalf("list waited on its own SQLite connection: %v", err)
	}
}

func TestCreateOnlyAccountCannotReplaceAnExistingCredential(t *testing.T) {
	service := newTestService(t)
	_, err := service.Upsert(t.Context(), UpsertRequest{Platform: PlatformWeibo, AccountID: "existing", Label: "original", Cookie: "SUB=original", Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.Upsert(t.Context(), UpsertRequest{CreateOnly: true, Platform: PlatformWeibo, AccountID: "existing", Label: "replacement", Cookie: "SUB=replacement"})
	if !errors.Is(err, ErrAccountAlreadyExists) {
		t.Fatalf("duplicate create=%v", err)
	}
	account, err := service.Get(t.Context(), PlatformWeibo, "existing")
	if err != nil {
		t.Fatal(err)
	}
	cookie, err := service.ReadCookie(t.Context(), account)
	if err != nil || account.Label != "original" || cookie != "SUB=original" {
		t.Fatalf("existing account was changed: label=%s err=%v", account.Label, err)
	}
	results := make(chan error, 8)
	var pending sync.WaitGroup
	for index := range 8 {
		pending.Go(func() {
			_, err := service.Upsert(t.Context(), UpsertRequest{CreateOnly: true, Platform: PlatformWeibo, AccountID: "concurrent", Label: fmt.Sprintf("attempt-%d", index)})
			results <- err
		})
	}
	pending.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrAccountAlreadyExists) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatalf("create-only winners=%d", success)
	}
}

func TestListEnabledExcludesInvalidAndDisabledAccounts(t *testing.T) {
	service := newTestService(t)
	for _, entry := range []struct {
		id      string
		enabled bool
		state   string
	}{
		{"usable", true, CredentialValid}, {"invalid", true, CredentialInvalid}, {"disabled", false, CredentialValid},
	} {
		_, err := service.Upsert(t.Context(), UpsertRequest{Platform: PlatformWeibo, AccountID: entry.id, Enabled: entry.enabled, Cookie: "SUB=fixture", Credential: CredentialStatus{State: entry.state}})
		if err != nil {
			t.Fatal(err)
		}
	}
	items, err := service.ListEnabled(t.Context(), PlatformWeibo)
	if err != nil || len(items) != 1 || items[0].AccountID != "usable" {
		t.Fatalf("enabled accounts=%#v err=%v", items, err)
	}
	page, err := service.ListPage(t.Context(), pagination.Query{Limit: 100})
	if err != nil || page.Total != 3 {
		t.Fatalf("management page=%#v err=%v", page, err)
	}
}
