package main

import (
	"flag"
	"fmt"
	"time"

	"encoding/json"

	"github.com/asticode/go-astilectron"
	"github.com/asticode/go-astilectron-bootstrap"
	"github.com/asticode/go-astilog"
	"github.com/pkg/errors"
)

// Constants
const htmlAbout = `Welcome on <b>GoSnake</b> !<br>
This is using the bootstrap and the bundler.`

// Vars
var (
	AppName string
	BuiltAt string
	debug   = flag.Bool("d", true, "enables the debug mode")
	w       *astilectron.Window
)

func main() {
	// Init
	flag.Parse()
	astilog.FlagInit()

	// Run bootstrap
	astilog.Debugf("Running app built at %s", BuiltAt)
	options := bootstrap.Options{
		Asset:    Asset,
		AssetDir: AssetDir,
		AstilectronOptions: astilectron.Options{
			AppName:            AppName,
			AppIconDarwinPath:  "resources/icon.icns",
			AppIconDefaultPath: "resources/icon.png",
		},
		Debug: *debug,
		MenuOptions: []*astilectron.MenuItemOptions{{
			Label: astilectron.PtrStr("File"),
			SubMenu: []*astilectron.MenuItemOptions{{
				Label: astilectron.PtrStr("About"),
				OnClick: func(e astilectron.Event) (deleteListener bool) {
					if err := bootstrap.SendMessage(w, "about", htmlAbout, func(m *bootstrap.MessageIn) {
						// Unmarshal payload
						var s string
						if err := json.Unmarshal(m.Payload, &s); err != nil {
							astilog.Error(errors.Wrap(err, "unmarshaling payload failed"))
							return
						}
						astilog.Infof("About modal has been displayed and payload is %s!", s)
					}); err != nil {
						astilog.Error(errors.Wrap(err, "sending about event failed"))
					}
					return
				},
			}, {
				Role: astilectron.MenuItemRoleClose,
			}},
		}},
		OnWait: func(_ *astilectron.Astilectron, ws []*astilectron.Window, _ *astilectron.Menu, _ *astilectron.Tray, _ *astilectron.Menu) error {
			w = ws[0]
			go func() {
				time.Sleep(5 * time.Second)
				if err := bootstrap.SendMessage(w, "check.out.menu", "Don't forget to check out the menu!"); err != nil {
					astilog.Error(errors.Wrap(err, "sending check.out.menu event failed"))
				}
			}()
			return nil
		},
		RestoreAssets: RestoreAssets,
		Windows: []*bootstrap.Window{{
			Homepage:       "index.html",
			MessageHandler: handleMessages,
			Options: &astilectron.WindowOptions{
				BackgroundColor: astilectron.PtrStr("#333"),
				Center:          astilectron.PtrBool(true),
				Height:          astilectron.PtrInt(700),
				Width:           astilectron.PtrInt(700),
			},
		}},
	}
	if err := bootstrap.Run(options); err != nil {
		astilog.Fatal(errors.Wrap(err, "running bootstrap failed"))
	}
}

type Resp struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
func handleMessages(_ *astilectron.Window, m bootstrap.MessageIn) (payload interface{}, err error) {
	fmt.Println("handleMessages", m.Name)
	payload = Resp{
		Name: "hello",
		Path: "world",
	}
	return
}
