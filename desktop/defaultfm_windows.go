package main

import (
	"errors"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"github.com/junkerderprovinz/cannoncommander/backend/pkg/desktop"
)

// verb is the app's entry in the context menu of folders and drives, and
// becomes their default, so a double click on a folder anywhere in Windows
// opens it here. Folder itself stays with Explorer, since it also stands
// for places such as the recycle bin and the control panel.
const verb = "cannoncommander"

// openNewWindow is where Win+E looks for the program it starts, as Directory
// Opus and XYplorer use it.
const openNewWindow = `CLSID\{52205fd8-5dfb-447d-801a-d0b52f2e83e1}\shell\opennewwindow`

// previousValue keeps the verb that was the default before, to give the
// place back to.
const previousValue = "CannonCommanderPrevious"

var folderClasses = []string{"Directory", "Drive"}

// registryHandler writes the person's own classes below HKCU, which win over
// the machine's and need no administrator.
type registryHandler struct {
	classes string
}

func newFolderHandler() folderHandler { return registryHandler{classes: `Software\Classes`} }

func (h registryHandler) set(exe string, w desktop.Words) error {
	label := w.Open
	if label == "" {
		label = "CannonCommander"
	}
	for _, class := range folderClasses {
		shell := h.classes + `\` + class + `\shell`
		k, _, err := registry.CreateKey(registry.CURRENT_USER, shell, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		before, _, err := k.GetStringValue("")
		if err != nil && !errors.Is(err, registry.ErrNotExist) {
			k.Close()
			return err
		}
		err = k.SetStringValue("", verb)
		k.Close()
		if err != nil {
			return err
		}
		v, _, err := registry.CreateKey(registry.CURRENT_USER, shell+`\`+verb, registry.ALL_ACCESS)
		if err != nil {
			return err
		}
		err = errors.Join(v.SetStringValue("", label), v.SetStringValue("Icon", `"`+exe+`",0`))
		// A second switch on keeps what came before the first.
		if _, _, e := v.GetStringValue(previousValue); errors.Is(e, registry.ErrNotExist) && before != verb {
			err = errors.Join(err, v.SetStringValue(previousValue, before))
		}
		v.Close()
		if err != nil {
			return err
		}
		if err := setDefault(h.classes+`\`+class+`\shell\`+verb+`\command`, `"`+exe+`" "%V"`); err != nil {
			return err
		}
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, h.classes+`\`+openNewWindow+`\command`, registry.ALL_ACCESS)
	if err != nil {
		return err
	}
	// An empty DelegateExecute makes Explorer run the command instead of its
	// own handler.
	err = errors.Join(k.SetStringValue("", `"`+exe+`" `+homeFlag), k.SetStringValue("DelegateExecute", ""))
	k.Close()
	if err != nil {
		return err
	}
	assocChanged()
	return nil
}

func (h registryHandler) unset() error {
	var errs []error
	for _, class := range folderClasses {
		shell := h.classes + `\` + class + `\shell`
		before := ""
		if v, err := registry.OpenKey(registry.CURRENT_USER, shell+`\`+verb, registry.QUERY_VALUE); err == nil {
			before, _, _ = v.GetStringValue(previousValue)
			v.Close()
		}
		if k, err := registry.OpenKey(registry.CURRENT_USER, shell, registry.ALL_ACCESS); err == nil {
			if cur, _, _ := k.GetStringValue(""); cur == verb {
				if before == "" {
					err = k.DeleteValue("")
				} else {
					err = k.SetStringValue("", before)
				}
				errs = append(errs, err)
			}
			k.Close()
		}
		errs = append(errs, deleteTree(shell+`\`+verb))
		pruneEmpty(shell, h.classes+`\`+class)
	}
	errs = append(errs, deleteTree(h.classes+`\`+openNewWindow))
	parts := strings.Split(openNewWindow, `\`)
	for i := len(parts) - 1; i > 0; i-- {
		pruneEmpty(h.classes + `\` + strings.Join(parts[:i], `\`))
	}
	assocChanged()
	return errors.Join(errs...)
}

func (h registryHandler) isSet(exe string) bool {
	for _, class := range folderClasses {
		k, err := registry.OpenKey(registry.CURRENT_USER, h.classes+`\`+class+`\shell`, registry.QUERY_VALUE)
		if err != nil {
			return false
		}
		cur, _, _ := k.GetStringValue("")
		k.Close()
		if cur != verb {
			return false
		}
		c, err := registry.OpenKey(registry.CURRENT_USER, h.classes+`\`+class+`\shell\`+verb+`\command`, registry.QUERY_VALUE)
		if err != nil {
			return false
		}
		cmd, _, _ := c.GetStringValue("")
		c.Close()
		if !strings.HasPrefix(strings.ToLower(cmd), strings.ToLower(`"`+exe+`"`)) {
			return false
		}
	}
	return true
}

func setDefault(path, value string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, path, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	return k.SetStringValue("", value)
}

// deleteTree removes a key below HKCU with everything below it.
func deleteTree(path string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, path, registry.ENUMERATE_SUB_KEYS)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	subs, err := k.ReadSubKeyNames(-1)
	k.Close()
	if err != nil {
		return err
	}
	for _, s := range subs {
		if err := deleteTree(path + `\` + s); err != nil {
			return err
		}
	}
	return registry.DeleteKey(registry.CURRENT_USER, path)
}

// pruneEmpty removes the keys at paths, in order, as long as each holds
// neither keys nor values, so switching off leaves nothing behind that was
// not there before.
func pruneEmpty(paths ...string) {
	for _, p := range paths {
		k, err := registry.OpenKey(registry.CURRENT_USER, p, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		info, err := k.Stat()
		k.Close()
		if err != nil || info.SubKeyCount > 0 || info.ValueCount > 0 {
			return
		}
		if registry.DeleteKey(registry.CURRENT_USER, p) != nil {
			return
		}
	}
}

var procChangeNotify = windows.NewLazySystemDLL("shell32.dll").NewProc("SHChangeNotify")

// assocChanged tells Explorer that the file associations changed, so it reads
// them anew.
func assocChanged() {
	const shcneAssocChanged, shcnfIDList = 0x08000000, 0
	_, _, _ = procChangeNotify.Call(shcneAssocChanged, shcnfIDList, 0, 0)
}
