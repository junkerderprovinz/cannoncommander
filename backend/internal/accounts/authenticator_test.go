package accounts_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/go-webauthn/webauthn/protocol/webauthncbor"
	"github.com/go-webauthn/webauthn/protocol/webauthncose"
)

const passkeyOrigin = "https://cannon.lan:8443"

var b64 = base64.RawURLEncoding

// authenticator is a passkey in software: it answers the options of the
// server as a browser with a platform authenticator would.
type authenticator struct {
	t      *testing.T
	key    *ecdsa.PrivateKey
	id     []byte
	user   []byte
	count  uint32
	origin string
}

func newAuthenticator(t *testing.T) *authenticator {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	return &authenticator{t: t, key: key, id: id, origin: passkeyOrigin}
}

type publicKeyOptions struct {
	PublicKey struct {
		Challenge string `json:"challenge"`
		RP        struct {
			ID string `json:"id"`
		} `json:"rp"`
		RPID string `json:"rpId"`
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	} `json:"publicKey"`
}

func (a *authenticator) clientData(kind, challenge string) []byte {
	raw, err := json.Marshal(map[string]any{"type": kind, "challenge": challenge, "origin": a.origin, "crossOrigin": false})
	if err != nil {
		a.t.Fatal(err)
	}
	return raw
}

func (a *authenticator) authData(rpID string, flags byte, attested []byte) []byte {
	rp := sha256.Sum256([]byte(rpID))
	out := append(rp[:], flags)
	out = binary.BigEndian.AppendUint32(out, a.count)
	return append(out, attested...)
}

// create answers navigator.credentials.create.
func (a *authenticator) create(options json.RawMessage) map[string]any {
	var o publicKeyOptions
	if err := json.Unmarshal(options, &o); err != nil {
		a.t.Fatal(err)
	}
	user, err := b64.DecodeString(o.PublicKey.User.ID)
	if err != nil {
		a.t.Fatal(err)
	}
	a.user = user
	cose, err := webauthncbor.Marshal(webauthncose.EC2PublicKeyData{
		PublicKeyData: webauthncose.PublicKeyData{KeyType: int64(webauthncose.EllipticKey), Algorithm: int64(webauthncose.AlgES256)},
		Curve:         1,
		XCoord:        a.key.X.FillBytes(make([]byte, 32)),
		YCoord:        a.key.Y.FillBytes(make([]byte, 32)),
	})
	if err != nil {
		a.t.Fatal(err)
	}
	attested := make([]byte, 16)
	attested = binary.BigEndian.AppendUint16(attested, uint16(len(a.id)))
	attested = append(append(attested, a.id...), cose...)
	// Present, verified, with attested credential data.
	data := a.authData(o.PublicKey.RP.ID, 0x45, attested)
	object, err := webauthncbor.Marshal(map[string]any{"fmt": "none", "attStmt": map[string]any{}, "authData": data})
	if err != nil {
		a.t.Fatal(err)
	}
	return map[string]any{
		"id": b64.EncodeToString(a.id), "rawId": b64.EncodeToString(a.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64.EncodeToString(a.clientData("webauthn.create", o.PublicKey.Challenge)),
			"attestationObject": b64.EncodeToString(object),
		},
	}
}

// get answers navigator.credentials.get.
func (a *authenticator) get(options json.RawMessage) map[string]any {
	var o publicKeyOptions
	if err := json.Unmarshal(options, &o); err != nil {
		a.t.Fatal(err)
	}
	a.count++
	data := a.authData(o.PublicKey.RPID, 0x05, nil)
	client := a.clientData("webauthn.get", o.PublicKey.Challenge)
	sum := sha256.Sum256(client)
	digest := sha256.Sum256(append(append([]byte{}, data...), sum[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, a.key, digest[:])
	if err != nil {
		a.t.Fatal(err)
	}
	return map[string]any{
		"id": b64.EncodeToString(a.id), "rawId": b64.EncodeToString(a.id), "type": "public-key",
		"response": map[string]any{
			"clientDataJSON":    b64.EncodeToString(client),
			"authenticatorData": b64.EncodeToString(data),
			"signature":         b64.EncodeToString(sig),
			"userHandle":        b64.EncodeToString(a.user),
		},
	}
}
