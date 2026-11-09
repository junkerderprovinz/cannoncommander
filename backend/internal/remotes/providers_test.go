package remotes

import (
	"slices"
	"strings"
	"testing"
)

func TestTheCatalogNamesProductsAndOffersEveryOtherBackend(t *testing.T) {
	f := newFixture(t)
	var out Catalog
	if code := f.call("anna", "GET", "/remotes/types?advanced=true", nil, &out); code != 200 {
		t.Fatalf("types: %d", code)
	}
	var ids []string
	for _, p := range out.Providers {
		ids = append(ids, p.ID)
	}
	// Clouds first, then storage, then protocols, each by name.
	want := "gdrive,nextcloud,opencloud,owncloud,oxicloud,azureblob,memory,sftp,smb,webdav"
	if strings.Join(ids, ",") != want {
		t.Fatalf("providers %v", ids)
	}
	if strings.Join(out.Unlisted, ",") != "zzz" {
		t.Fatalf("unlisted %v", out.Unlisted)
	}
	gdrive := out.Providers[0]
	if gdrive.Auth != AuthOAuth || gdrive.Mark != "IconGoogleDrive" {
		t.Fatalf("gdrive %+v", gdrive)
	}
}

func TestOptionsAskTheEssentialFirstAndHideWhatRcloneCallsSensitive(t *testing.T) {
	f := newFixture(t)
	var out Catalog
	f.call("anna", "GET", "/remotes/types?advanced=true", nil, &out)
	byID := map[string]Type{}
	for _, ty := range out.Types {
		byID[ty.ID] = ty
	}
	var names []string
	for _, o := range byID["sftp"].Options {
		names = append(names, o.Name)
	}
	// key_file is advanced but essential, since a key and a password are the
	// two ways in.
	if strings.Join(names, ",") != "host,user,port,pass,key_file,host_keys" {
		t.Fatalf("sftp options %v", names)
	}
	secret := func(typ, name string) bool {
		i := slices.IndexFunc(byID[typ].Options, func(o Option) bool { return o.Name == name })
		return byID[typ].Options[i].Secret
	}
	if !secret("azureblob", "sas_url") || secret("azureblob", "account") || !secret("webdav", "bearer_token") || secret("webdav", "url") {
		t.Fatal("sensitive options are shown or the account is hidden")
	}
	if help := byID["webdav"].Options[0].Help; help != "URL of http host to connect to." {
		t.Fatalf("help %q", help)
	}
}

func TestASavedRemoteNamesItsProduct(t *testing.T) {
	for _, c := range []struct {
		typ    string
		params map[string]string
		want   string
	}{
		{"webdav", map[string]string{"vendor": "nextcloud"}, "nextcloud"},
		{"webdav", map[string]string{"vendor": "other"}, "webdav"},
		{"webdav", map[string]string{"vendor": "other", "description": "OxiCloud"}, "oxicloud"},
		{"s3", map[string]string{"provider": "Minio"}, "minio"},
		{"s3", map[string]string{"provider": "AWS"}, "s3"},
		{"drive", nil, "gdrive"},
		{"tardigrade", nil, "storj"},
		{"zzz", nil, ""},
	} {
		if got := providerFor(c.typ, c.params); got != c.want {
			t.Errorf("%s %v: %q, want %q", c.typ, c.params, got, c.want)
		}
	}
}

func TestTheListNamesTheProductToEverybodyAndTheParametersToAdmins(t *testing.T) {
	f := newFixture(t)
	f.call("jdp", "POST", "/remotes", Input{Name: "cloud", Type: "webdav", Params: map[string]string{"url": "https://c.example", "vendor": "nextcloud", "pass": "geheim"}}, nil)
	var list List
	f.call("anna", "GET", "/remotes", nil, &list)
	if r := list.Remotes[0]; r.Provider != "nextcloud" || r.Params != nil || r.Secrets != nil {
		t.Fatalf("anna sees %+v", r)
	}
	f.call("jdp", "GET", "/remotes", nil, &list)
	if r := list.Remotes[0]; r.Params["vendor"] != "nextcloud" || !slices.Equal(r.Secrets, []string{"pass"}) {
		t.Fatalf("jdp sees %+v", r)
	}
}
