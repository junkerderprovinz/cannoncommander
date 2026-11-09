package remotes

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/crypto/ssh"
)

// hostKeysKey is rclone's sftp option that pins the server's key. Without a
// pin or a known_hosts_file rclone talks to whichever server answers.
const hostKeysKey = "host_keys"

// HostKey is the key an SSH server proves itself with.
type HostKey struct {
	// Pin is the key as host_keys takes it.
	Pin string `json:"pin"`
	// Fingerprint is the SHA256 form that providers publish and ssh prints.
	Fingerprint string `json:"fingerprint"`
}

const hostKeyWait = 15 * time.Second

var errGotHostKey = errors.New("host key read")

// readHostKey asks an SSH server for its key and hangs up before signing
// in. rclone only offers the algorithms of the keys pinned, so the one key
// read here is all a remote needs.
func readHostKey(ctx context.Context, host, port string) (HostKey, error) {
	if port == "" {
		port = "22"
	}
	ctx, cancel := context.WithTimeout(ctx, hostKeyWait)
	defer cancel()
	addr := net.JoinHostPort(host, port)
	conn, err := new(net.Dialer).DialContext(ctx, "tcp", addr)
	if err != nil {
		return HostKey{}, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}
	var got ssh.PublicKey
	_, _, _, err = ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User: "cannoncommander",
		HostKeyAlgorithms: []string{
			ssh.KeyAlgoED25519, ssh.KeyAlgoECDSA256, ssh.KeyAlgoECDSA384, ssh.KeyAlgoECDSA521,
			ssh.KeyAlgoRSASHA512, ssh.KeyAlgoRSASHA256,
		},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			got = key
			return errGotHostKey
		},
	})
	if got == nil {
		return HostKey{}, err
	}
	return HostKey{
		Pin:         got.Type() + " " + base64.StdEncoding.EncodeToString(got.Marshal()),
		Fingerprint: ssh.FingerprintSHA256(got),
	}, nil
}

// pinnedKey is the first key of a host_keys value, for its fingerprint.
func pinnedKey(pins string) (HostKey, bool) {
	first, _, _ := strings.Cut(pins, ",")
	first = strings.TrimSpace(first)
	_, encoded, ok := strings.Cut(first, " ")
	if !ok {
		return HostKey{}, false
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimSpace(encoded))
	if err != nil {
		return HostKey{}, false
	}
	key, err := ssh.ParsePublicKey(raw)
	if err != nil {
		return HostKey{}, false
	}
	return HostKey{Pin: first, Fingerprint: ssh.FingerprintSHA256(key)}, true
}

// checksHostKey reports whether sftp parameters already say how to check
// the server: a pin, a known_hosts file (or none, said on purpose), or an
// ssh command that checks for itself.
func checksHostKey(params map[string]string) bool {
	return params[hostKeysKey] != "" || params["known_hosts_file"] != "" || params["ssh"] != ""
}

// pinSaved pins a saved sftp remote that does not say how to check its
// server to the key the server shows now, so one saved without a test is
// held to its key from the first mount on.
func (s *Service) pinSaved(ctx context.Context, name string) error {
	sec, err := s.section(ctx, name)
	if err != nil {
		return err
	}
	if sec["type"] != "sftp" || sec["host"] == "" || checksHostKey(sec) {
		return nil
	}
	key, err := s.readKey(ctx, sec["host"], sec["port"])
	if err != nil {
		return fmt.Errorf("read the host key of %s: %w", sec["host"], err)
	}
	if err := s.updateSection(ctx, name, map[string]string{hostKeysKey: key.Pin}, nil, "", nil); err != nil {
		return err
	}
	logger.Infof("remotes: pinned %s to %s", name, key.Fingerprint)
	return nil
}
