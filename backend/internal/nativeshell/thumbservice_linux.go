package nativeshell

import (
	"context"
	"errors"
	"slices"

	"github.com/godbus/dbus/v5"
)

// thumbService is the freedesktop thumbnail service, which tumbler offers
// and which draws into the cache that file managers share.
const (
	thumbService = "org.freedesktop.thumbnails.Thumbnailer1"
	thumbPath    = "/org/freedesktop/thumbnails/Thumbnailer1"
)

var errServiceFailed = errors.New("the thumbnail service could not draw the file")

// askThumbService asks the service to draw the thumbnail of uri at flavor,
// a folder of the cache such as normal or large, and waits until it is done
// with it.
func askThumbService(ctx context.Context, uri, mime, flavor string) error {
	conn, err := connect()
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := conn.AddMatchSignal(dbus.WithMatchInterface(thumbService), dbus.WithMatchObjectPath(thumbPath)); err != nil {
		return err
	}
	signals := make(chan *dbus.Signal, 16)
	conn.Signal(signals)
	var handle uint32
	err = conn.Object(thumbService, thumbPath).CallWithContext(ctx, thumbService+".Queue", 0, []string{uri}, []string{mime}, flavor, "foreground", uint32(0)).Store(&handle)
	if err != nil {
		return err
	}
	failed := false
	for {
		select {
		case <-ctx.Done():
			conn.Object(thumbService, thumbPath).Call(thumbService+".Dequeue", dbus.FlagNoReplyExpected, handle)
			return ctx.Err()
		case s := <-signals:
			if len(s.Body) == 0 || s.Body[0] != handle {
				continue
			}
			switch s.Name {
			case thumbService + ".Error":
				if uris, ok := s.Body[1].([]string); ok && slices.Contains(uris, uri) {
					failed = true
				}
			case thumbService + ".Finished":
				if failed {
					return errServiceFailed
				}
				return nil
			}
		}
	}
}
