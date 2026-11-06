// Package shares creates share links and serves the public share pages.
package shares

import (
	_ "embed"
	"encoding/json"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	v1 "github.com/junkerderprovinz/cannoncommander/backend/internal/api/v1"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/users"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/utils"
)

//go:embed openapi.yaml
var contract []byte

// Routes registers the contract and, when there is a database, the
// handlers.
func Routes(r *v1.Router) {
	r.Contract("shares", contract)
	s := For(r.Deps())
	if s == nil {
		return
	}
	r.Handle("GET /shares", s.mine)
	r.Handle("POST /shares", s.create)
	r.Handle("GET /shares/{id}", s.get)
	r.Handle("GET /shares/{id}/qr", s.qr)
	r.Handle("PATCH /shares/{id}", s.update)
	r.Handle("DELETE /shares/{id}", s.delete)
	r.Handle("GET /admin/shares", s.all)
	r.Handle("GET /s/{slug}", s.publicGet)
	r.Handle("POST /s/{slug}/unlock", s.publicUnlock)
	r.Handle("GET /s/{slug}/list", s.publicList)
	r.Handle("GET /s/{slug}/raw", s.publicRaw)
	r.Handle("POST /s/{slug}/upload", s.publicUpload)
	r.Handle("GET /s/{slug}/upload/{id}", s.publicUploadGet)
	r.Handle("PUT /s/{slug}/upload/{id}", s.publicUploadChunk)
	r.Handle("DELETE /s/{slug}/upload/{id}", s.publicUploadCancel)
}

type createRequest struct {
	Paths []string `json:"paths"`
	// Name is the folder name of a collection.
	Name    string `json:"name"`
	Message string `json:"message"`
	// Slug is the end of the address. Empty picks one that is hard to guess.
	Slug        string     `json:"slug"`
	Expires     *time.Time `json:"expires"`
	Password    string     `json:"password"`
	Limit       int        `json:"limit"`
	Uploads     bool       `json:"uploads"`
	UploadLimit int64      `json:"uploadLimit"`
	Notify      bool       `json:"notify"`
}

// optionalTime tells a missing expiry, which keeps the old one, from null,
// which means the link never expires.
type optionalTime struct {
	set bool
	t   *time.Time
}

func (o *optionalTime) UnmarshalJSON(b []byte) error {
	o.set = true
	if string(b) == "null" {
		o.t = nil
		return nil
	}
	var t time.Time
	if err := json.Unmarshal(b, &t); err != nil {
		return err
	}
	o.t = &t
	return nil
}

type updateRequest struct {
	Slug *string `json:"slug"`
	// Reroll gives the link a fresh address. The old one leads nowhere.
	Reroll  bool         `json:"reroll"`
	Name    *string      `json:"name"`
	Message *string      `json:"message"`
	Expires optionalTime `json:"expires"`
	// Password sets a new password, an empty one removes it.
	Password    *string `json:"password"`
	Limit       *int    `json:"limit"`
	Uploads     *bool   `json:"uploads"`
	UploadLimit *int64  `json:"uploadLimit"`
	Notify      *bool   `json:"notify"`
}

func mayShare(u *users.User) bool { return u.Permissions.Share || u.Permissions.Admin }

var errMayNotShare = v1.Coded(http.StatusForbidden, "mayNotShare", "this account may not share files")

func (s *Service) mine(w http.ResponseWriter, r *http.Request) error {
	list, err := s.list(r.Context(), `WHERE owner_id = ?`, ownerKey(v1.UserFrom(r).ID))
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, list)
}

func (s *Service) all(w http.ResponseWriter, r *http.Request) error {
	list, err := s.list(r.Context(), ``)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, list)
}

func (s *Service) get(w http.ResponseWriter, r *http.Request) error {
	sh, err := s.owned(r)
	if err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, sh)
}

// owned returns the link of the request, for its owner and for admins.
func (s *Service) owned(r *http.Request) (*Share, error) {
	u := v1.UserFrom(r)
	sh, err := s.one(r.Context(), `WHERE id = ?`, r.PathValue("id"))
	if err != nil {
		return nil, err
	}
	if sh.OwnerID != u.ID && !u.Permissions.Admin {
		return nil, v1.Coded(http.StatusNotFound, "noLink", "there is no such link")
	}
	return sh, nil
}

func (s *Service) create(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	if !mayShare(u) {
		return errMayNotShare
	}
	var req createRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	sh := &Share{ID: newID(), OwnerID: u.ID, Owner: u.Username, Message: strings.TrimSpace(req.Message), Expires: req.Expires, Limit: req.Limit,
		Uploads: req.Uploads, UploadLimit: req.UploadLimit, Notify: req.Notify}
	var err error
	if sh.Paths, sh.Kind, err = s.checkPaths(u, req.Paths); err != nil {
		return err
	}
	sh.Name = strings.TrimSpace(req.Name)
	if sh.Kind == KindCollection {
		if !ccfs.ValidName(sh.Name) {
			return v1.Coded(http.StatusBadRequest, "collectionName", "give the collection a name, it becomes the folder the other side sees")
		}
	} else {
		sh.Name = path.Base(sh.Paths[0])
	}
	if req.Password != "" {
		if err := setPassword(sh, req.Password); err != nil {
			return err
		}
	}
	if err := s.checkSettings(u, sh); err != nil {
		return err
	}
	if sh.Slug, err = s.pickSlug(r, req.Slug, sh.Name); err != nil {
		return err
	}
	if err := s.insert(r.Context(), sh); err != nil {
		return err
	}
	if sh, err = s.one(r.Context(), `WHERE id = ?`, sh.ID); err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusCreated, sh)
}

func (s *Service) update(w http.ResponseWriter, r *http.Request) error {
	u := v1.UserFrom(r)
	sh, err := s.owned(r)
	if err != nil {
		return err
	}
	if !mayShare(u) {
		return errMayNotShare
	}
	var req updateRequest
	if err := v1.DecodeJSON(r, &req); err != nil {
		return err
	}
	switch {
	case req.Reroll:
		if sh.Slug, err = s.freshSlug(r.Context(), sh.Name); err != nil {
			return err
		}
	case req.Slug != nil && *req.Slug != sh.Slug:
		if sh.Slug, err = s.pickSlug(r, *req.Slug, sh.Name); err != nil {
			return err
		}
	}
	if req.Name != nil && sh.Kind == KindCollection {
		if sh.Name = strings.TrimSpace(*req.Name); !ccfs.ValidName(sh.Name) {
			return v1.Coded(http.StatusBadRequest, "collectionName", "give the collection a name, it becomes the folder the other side sees")
		}
	}
	if req.Message != nil {
		sh.Message = strings.TrimSpace(*req.Message)
	}
	if req.Expires.set {
		sh.Expires = req.Expires.t
	}
	if req.Password != nil {
		sh.passwordHash = ""
		if *req.Password != "" {
			if err := setPassword(sh, *req.Password); err != nil {
				return err
			}
		}
	}
	if req.Limit != nil {
		sh.Limit = *req.Limit
	}
	if req.Uploads != nil {
		sh.Uploads = *req.Uploads
	}
	if req.UploadLimit != nil {
		sh.UploadLimit = *req.UploadLimit
	}
	if req.Notify != nil {
		sh.Notify = *req.Notify
	}
	// An admin who edits a link of someone else checks the uploads against
	// the owner, whose rights the link uses.
	owner := u
	if sh.OwnerID != u.ID {
		if owner, err = UserByID(sh.OwnerID); err != nil {
			return v1.Coded(http.StatusConflict, "linkOwnerGone", "the account behind this link is gone")
		}
	}
	if err := s.checkSettings(owner, sh); err != nil {
		return err
	}
	if err := s.save(r.Context(), sh); err != nil {
		return err
	}
	if sh, err = s.one(r.Context(), `WHERE id = ?`, sh.ID); err != nil {
		return err
	}
	return v1.WriteJSON(w, r, http.StatusOK, sh)
}

// delete asks for no share right: withdrawing a link only takes something
// away from its visitors, and someone who lost the right can still clean up.
func (s *Service) delete(w http.ResponseWriter, r *http.Request) error {
	sh, err := s.owned(r)
	if err != nil {
		return err
	}
	if err := s.remove(r.Context(), sh.ID); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

// checkPaths makes sure u may hand out every path and tells what kind of
// link they make.
func (s *Service) checkPaths(u *users.User, paths []string) ([]string, string, error) {
	if len(paths) == 0 {
		return nil, "", v1.Coded(http.StatusBadRequest, "shareNothing", "choose a file or folder to share")
	}
	if len(paths) > maxPaths {
		return nil, "", v1.Coded(http.StatusBadRequest, "shareTooMuch", "that is too much for one link")
	}
	var out []string
	seen := map[string]bool{}
	var single os.FileInfo
	for _, p := range paths {
		loc, err := s.fs.Check(u, p, ccfs.View|ccfs.Download)
		if err != nil {
			return nil, "", err
		}
		if loc.InArchive() {
			return nil, "", v1.Coded(http.StatusBadRequest, "shareInArchive", "nothing can be shared from inside an archive")
		}
		if loc.Virtual() {
			return nil, "", v1.Coded(http.StatusBadRequest, "outsideSources", "only entries inside a source can be shared")
		}
		if err := s.fs.Follows(u, loc, ccfs.View|ccfs.Download); err != nil {
			return nil, "", err
		}
		info, err := os.Stat(loc.Real)
		if err != nil {
			return nil, "", err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return nil, "", v1.Coded(http.StatusBadRequest, "shareOnlyFiles", "only files and folders can be shared")
		}
		if seen[loc.Path] {
			continue
		}
		seen[loc.Path] = true
		out = append(out, loc.Path)
		single = info
	}
	switch {
	case len(out) > 1:
		return out, KindCollection, nil
	case single.IsDir():
		return out, KindFolder, nil
	}
	return out, KindFile, nil
}

func (s *Service) checkSettings(owner *users.User, sh *Share) error {
	if sh.Expires != nil && !sh.Expires.After(time.Now()) {
		return v1.Coded(http.StatusBadRequest, "expiryPast", "the expiry date lies in the past")
	}
	if sh.Limit < 0 || sh.UploadLimit < 0 {
		return v1.Errorf(http.StatusBadRequest, "limits cannot be negative")
	}
	if utf8.RuneCountInString(sh.Message) > maxMessage {
		return v1.Coded(http.StatusBadRequest, "messageLength", "the message can have at most 2000 characters").With("n", 2000)
	}
	if !sh.Uploads {
		return nil
	}
	if sh.Kind != KindFolder {
		return v1.Coded(http.StatusBadRequest, "uploadsOneFolder", "uploads only work for a link to one folder")
	}
	_, err := s.fs.Check(owner, sh.Paths[0], ccfs.Create)
	return err
}

func (s *Service) pickSlug(r *http.Request, slug, name string) (string, error) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	if slug == "" {
		return s.freshSlug(r.Context(), name)
	}
	if !slugPattern.MatchString(slug) {
		return "", v1.Coded(http.StatusBadRequest, "slugRule", "the address needs 3 to 40 characters: small letters, digits and dashes")
	}
	var taken int
	if err := s.db.QueryRowContext(r.Context(), `SELECT COUNT(*) FROM share_links WHERE slug = ?`, slug).Scan(&taken); err != nil {
		return "", err
	}
	if taken > 0 {
		return "", errSlugTaken
	}
	return slug, nil
}

func setPassword(sh *Share, pw string) error {
	if len([]rune(pw)) < minPassword {
		return v1.Coded(http.StatusBadRequest, "linkPasswordShort", "the password needs at least 6 characters").With("n", 6)
	}
	hash, err := utils.HashPwd(pw)
	if err != nil {
		return err
	}
	sh.passwordHash = hash
	return nil
}
