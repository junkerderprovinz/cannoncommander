package system

import (
	"context"
	"testing"
)

func TestConnectingAnOfficeServerSavesItWhenItAnswers(t *testing.T) {
	f := newFixture(t)
	res, o, err := f.svc.connectOffice(context.Background(), Office{Kind: OfficeEuroOffice, URL: "http://tower.lan:9900/", Secret: "geheim"})
	if err != nil || !res.OK {
		t.Fatalf("%+v, %v", res, err)
	}
	if len(f.tested) != 1 || f.tested[0] != "eurooffice http://tower.lan:9900 geheim" {
		t.Fatalf("tested %v", f.tested)
	}
	if o.Kind != OfficeEuroOffice || o.URL != "http://tower.lan:9900" || !o.SecretSet || o.Secret != "" {
		t.Fatalf("office %+v, want it saved without the secret in the answer", o)
	}
	saved, _ := f.svc.store.get(context.Background())
	if saved.Office.Secret != "geheim" || len(f.applied) != 1 {
		t.Fatalf("saved %+v, applied %d", saved.Office, len(f.applied))
	}
}

func TestConnectingTheSameAddressKeepsTheSavedSecret(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	if _, _, err := f.svc.connectOffice(ctx, Office{Kind: OfficeOnlyOffice, URL: "http://tower.lan:9900", Secret: "alt"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := f.svc.connectOffice(ctx, Office{Kind: OfficeOnlyOffice, URL: "http://tower.lan:9900"}); err != nil {
		t.Fatal(err)
	}
	saved, _ := f.svc.store.get(ctx)
	if saved.Office.Secret != "alt" || f.tested[1] != "onlyoffice http://tower.lan:9900 alt" {
		t.Fatalf("saved %+v, tested %v", saved.Office, f.tested)
	}
	if _, _, err := f.svc.connectOffice(ctx, Office{Kind: OfficeCollabora, URL: "https://tower.lan:9980"}); err != nil {
		t.Fatal(err)
	}
	if saved, _ = f.svc.store.get(ctx); saved.Office.Secret != "" {
		t.Fatalf("another server kept the secret %q", saved.Office.Secret)
	}
}

func TestAnOfficeServerThatDoesNotAnswerIsNotSaved(t *testing.T) {
	f := newFixture(t)
	f.svc.test = func(_ context.Context, kind, _, _ string) IntegrationResult {
		return IntegrationResult{Kind: kind, Error: "connection refused"}
	}
	res, o, err := f.svc.connectOffice(context.Background(), Office{Kind: OfficeOnlyOffice, URL: "http://tower.lan:9900", Secret: "x"})
	if err != nil || res.Reachable || o.Kind != OfficeNone {
		t.Fatalf("%+v, %+v, %v", res, o, err)
	}
	if saved, _ := f.svc.store.get(context.Background()); saved.Office.Kind != OfficeNone || len(f.applied) != 0 {
		t.Fatalf("saved %+v", saved.Office)
	}
}
