package remotes

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
)

func (s *Service) checkName(name string) error {
	switch {
	case name == "":
		return v1.Errorf(http.StatusBadRequest, "give the remote a name")
	case strings.HasPrefix(name, tryPrefix) || !validName.MatchString(name):
		return v1.Errorf(http.StatusBadRequest, "a name has small letters, digits, _ and - only")
	}
	return nil
}

// checkMount checks a mount folder: below Root, as in /remotes/name, and
// not inside or around the folder of another remote.
func (s *Service) checkMount(dir, self string, rows []row) error {
	if !filepath.IsAbs(dir) || dir == s.cfg.Root || !within(dir, s.cfg.Root) {
		return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("the folder has to lie below %s, such as %s", s.cfg.Root, filepath.Join(s.cfg.Root, "name")))
	}
	for _, r := range rows {
		if r.Name == self {
			continue
		}
		if within(dir, r.Mount) || within(r.Mount, dir) {
			return v1.Errorf(http.StatusBadRequest, dir+" is already taken by the remote "+r.Name)
		}
	}
	return nil
}

func within(p, base string) bool {
	rel, err := filepath.Rel(base, p)
	return err == nil && !strings.HasPrefix(rel, "..")
}

func (s *Service) obscure(ctx context.Context, clear string) (string, error) {
	var out struct {
		Obscured string `json:"obscured"`
	}
	err := s.rc.call(ctx, "core/obscure", map[string]string{"clear": clear}, &out)
	return out.Obscured, err
}

// params turns what the form sent into rclone parameters. Passwords are
// obscured here, so every call can tell rclone to take values as they are.
// token is the OAuth token among them, for the config questions, and picked
// holds the answers to questions an earlier test asked, which rclone's own
// setup names config_ and which are no parameters.
func (s *Service) params(ctx context.Context, p *provider, in map[string]string) (set map[string]string, unset []string, token string, picked map[string]string, err error) {
	set, picked = map[string]string{}, map[string]string{}
	for k, v := range in {
		k = strings.ToLower(strings.TrimSpace(k))
		if k == "" || k == "type" {
			continue
		}
		if _, isOption := p.option(k); !isOption && strings.HasPrefix(k, "config_") {
			if v != "" {
				picked[k] = v
			}
			continue
		}
		if v == "" {
			if !p.secret(k) {
				unset = append(unset, k)
			}
			continue
		}
		if k == "token" {
			if v, _, err = normalizeToken(v); err != nil {
				return nil, nil, "", nil, err
			}
			token = v
		}
		if o, ok := p.option(k); ok && o.IsPassword {
			if v, err = s.obscure(ctx, v); err != nil {
				return nil, nil, "", nil, err
			}
		}
		set[k] = v
	}
	return set, unset, token, picked, nil
}

// configState is rclone's answer to a config call that may have a question.
type configState struct {
	State  string `json:"State"`
	Error  string `json:"Error"`
	Option *struct {
		Name       string    `json:"Name"`
		Help       string    `json:"Help"`
		DefaultStr string    `json:"DefaultStr"`
		ValueStr   string    `json:"ValueStr"`
		Required   bool      `json:"Required"`
		Examples   []example `json:"Examples"`
	} `json:"Option"`
}

// Question is a choice rclone's setup leaves to the person, such as which of
// several drives a sign-in opens. The answer comes back as the parameter
// Name.
type Question struct {
	Name    string    `json:"name"`
	Help    string    `json:"help"`
	Choices []Example `json:"choices"`
}

// errAsks carries the question a setup stopped at.
type errAsks struct{ q *Question }

func (e errAsks) Error() string { return "rclone asks " + e.q.Name }

// asking makes a question the person answers from what rclone offers. A
// choice with one way to go is no question, and neither is yes or no, which
// rclone's defaults settle.
func asking(name, help string, examples []example) *Question {
	if len(examples) < 2 || !strings.HasPrefix(name, "config_") {
		return nil
	}
	q := &Question{Name: name, Help: firstLine(help)}
	yesNo := true
	for _, e := range examples {
		yesNo = yesNo && (e.Value == "true" || e.Value == "false")
		q.Choices = append(q.Choices, Example{Value: e.Value, Help: firstLine(e.Help)})
	}
	if yesNo {
		return nil
	}
	return q
}

// answers settle the questions a backend asks after it has its parameters.
// The token comes from rclone authorize on another computer, so nothing
// here may refresh it or wait for a browser, and it was made with rclone's
// shared client id.
var answers = map[string]string{
	"config_refresh_token":    "false",
	"config_is_local":         "false",
	"config_shared_client_id": "true",
}

// setupAnswers settle, per backend, choices whose usual answer is plain:
// OneDrive's own drive rather than a SharePoint search, Jottacloud's
// standard login.
var setupAnswers = map[string]map[string]string{
	"onedrive":   {"config_type": "onedrive"},
	"jottacloud": {"config_type": "standard", "config_non_standard": "false"},
}

// withSetupAnswers adds the plain answers for typ to what was picked, which
// wins.
func withSetupAnswers(typ string, picked map[string]string) map[string]string {
	out := maps.Clone(setupAnswers[typ])
	if out == nil {
		out = map[string]string{}
	}
	maps.Copy(out, picked)
	return out
}

// answer goes through rclone's config questions the way rclone config
// would with every default taken, except for a real choice, which it hands
// back as errAsks unless picked answers it.
func (s *Service) answer(ctx context.Context, name string, st configState, token string, picked map[string]string) error {
	for round := 0; st.State != ""; round++ {
		if st.Error != "" {
			return v1.Errorf(http.StatusBadRequest, "rclone: "+st.Error)
		}
		if round == 20 {
			return fmt.Errorf("rclone keeps asking questions about %s", name)
		}
		result := ""
		if o := st.Option; o != nil {
			a, known := answers[o.Name]
			chosen, given := picked[o.Name]
			switch {
			case known:
				result = a
			case given:
				result = chosen
			case o.Name == "config_token" && token != "":
				result = token
			case asking(o.Name, o.Help, o.Examples) != nil:
				return errAsks{asking(o.Name, o.Help, o.Examples)}
			case o.ValueStr != "":
				result = o.ValueStr
			case o.DefaultStr != "":
				result = o.DefaultStr
			case len(o.Examples) == 1:
				result = o.Examples[0].Value
			case o.Required:
				return v1.Errorf(http.StatusBadRequest, fmt.Sprintf("rclone needs %s: %s", o.Name, firstLine(o.Help)))
			}
		}
		next := configState{}
		err := s.rc.call(ctx, "config/update", map[string]any{
			"name": name, "parameters": map[string]string{},
			"opt": map[string]any{"nonInteractive": true, "noObscure": true, "continue": true, "state": st.State, "result": result},
		}, &next)
		if err != nil {
			return err
		}
		st = next
	}
	return nil
}

func (s *Service) createSection(ctx context.Context, name, typ string, params map[string]string, token string, picked map[string]string) error {
	var st configState
	err := s.rc.call(ctx, "config/create", map[string]any{
		"name": name, "type": typ, "parameters": params,
		"opt": map[string]any{"nonInteractive": true, "noObscure": true},
	}, &st)
	if err != nil {
		return err
	}
	return s.answer(ctx, name, st, token, picked)
}

func (s *Service) updateSection(ctx context.Context, name string, set map[string]string, unset []string, token string, picked map[string]string) error {
	if len(set) > 0 {
		var st configState
		err := s.rc.call(ctx, "config/update", map[string]any{
			"name": name, "parameters": set,
			"opt": map[string]any{"nonInteractive": true, "noObscure": true},
		}, &st)
		if err != nil {
			return err
		}
		if err := s.answer(ctx, name, st, token, picked); err != nil {
			return err
		}
	}
	if len(unset) > 0 {
		if err := s.rc.call(ctx, "config/unset", map[string]any{"name": name, "parameters": unset}, nil); err != nil {
			return err
		}
	}
	// rclone keeps a backend built from the old parameters otherwise.
	return s.rc.call(ctx, "fscache/clear", nil, nil)
}

// forget removes a section without waiting on the request that made it.
func (s *Service) forget(name string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.rc.call(ctx, "config/delete", map[string]string{"name": name}, nil); err != nil {
		logger.Errorf("remotes: remove %s: %v", name, err)
	}
	_ = s.rc.call(ctx, "fscache/clear", nil, nil)
}

// changes run detached from the request, so a closed browser tab does not
// leave rclone.conf and the table half changed.
func detached(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
}
