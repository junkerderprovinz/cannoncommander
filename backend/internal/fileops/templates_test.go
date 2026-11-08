package fileops_test

import (
	"net/http"
	"testing"
)

func TestTheContainerOffersNoTemplates(t *testing.T) {
	e := newEnv(t)
	var got struct {
		Templates []any `json:"templates"`
	}
	if code := e.call("anna", "GET", "/fs/templates", nil, &got); code != http.StatusOK || got.Templates == nil || len(got.Templates) != 0 {
		t.Errorf("templates %d %+v", code, got)
	}
	if code := e.call("anna", "POST", "/fs/new", map[string]string{"path": "/mnt/user/fotos/a.zip", "template": "shellnew:.zip"}, nil); code != http.StatusBadRequest {
		t.Errorf("new file in the container %d", code)
	}
}
