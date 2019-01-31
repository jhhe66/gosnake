package main

import (
	"flag"
	"net"
	"strconv"
	"time"

	"github.com/asticode/go-astilectron"
	"github.com/asticode/go-astilectron-bootstrap"
	"github.com/asticode/go-astilog"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	kcp "github.com/xtaci/kcp-go"

	"gosnake-server/comm"
	"gosnake/bindata"
)

// Vars
var (
	debug     = flag.Bool("d", false, "enables the debug mode")
	AppName   = "GoSnake"
	chKeyCode = make(chan int32, 10) // 传递用户操作按键给 engine
	chMode    = make(chan int, 1)    // 传递 mode 信号给 engine
	chExit    = make(chan int)       // 传递退出信号给 engine
	mainWin   *astilectron.Window
)

func main() {
	// Init
	flag.Parse()
	astilog.FlagInit()

	// Run bootstrap
	options := bootstrap.Options{
		Debug:         *debug,
		Asset:         bindata.Asset,
		AssetDir:      bindata.AssetDir,
		RestoreAssets: bindata.RestoreAssets,
		AstilectronOptions: astilectron.Options{
			AppName:            AppName,
			AppIconDarwinPath:  "resources/icon.icns",
			AppIconDefaultPath: "resources/icon.png",
		},
		MenuOptions: []*astilectron.MenuItemOptions{{
			Label: astilectron.PtrStr("File"),
			SubMenu: []*astilectron.MenuItemOptions{{
				Label: astilectron.PtrStr("About"),
				OnClick: func(e astilectron.Event) (deleteListener bool) {
					bootstrap.SendMessage(mainWin, "about", nil)
					return
				},
			}, {
				Role: astilectron.MenuItemRoleClose,
			}},
		}},
		Windows: []*bootstrap.Window{{
			Homepage:       "index.html",
			MessageHandler: handleMessages,
			Options: &astilectron.WindowOptions{
				BackgroundColor: astilectron.PtrStr("#fff"),
				Center:          astilectron.PtrBool(true),
				Height:          astilectron.PtrInt(500),
				Width:           astilectron.PtrInt(500),
			},
		}},
		OnWait: func(_ *astilectron.Astilectron, ws []*astilectron.Window, _ *astilectron.Menu, _ *astilectron.Tray, _ *astilectron.Menu) error {
			mainWin = ws[0]
			// Astilectron 已经 ready，启动 engine
			go engine()
			return nil
		},
		Adapter: func(a *astilectron.Astilectron) {
			// Astilectron 已经结束，通知 engine 退出
			a.On(astilectron.EventNameAppCrash, func(e astilectron.Event) (deleteListener bool) {
				close(chExit)
				return
			})
		},
	}
	if err := bootstrap.Run(options); err != nil {
		astilog.Fatal(errors.Wrap(err, "running bootstrap failed"))
	}
	time.Sleep(time.Second)
}

func handleMessages(_ *astilectron.Window, m bootstrap.MessageIn) (payload interface{}, err error) {
	// astilog.Debug("handleMessages:", m.Name, m.Payload)
	switch m.Name {
	case "start":
		// 从 Astilectron 收到 start 通知，转给 engine
		mode, err := strconv.Atoi(string(m.Payload))
		if err != nil || mode < 1 || mode > 3 {
			break
		}
		chMode <- mode

	case "keydown":
		// 从 Astilectron 收到用户按键操作，转给 engine
		kc, err := strconv.Atoi(string(m.Payload))
		if err != nil {
			break
		}
		chKeyCode <- int32(kc)
	}
	return
}

type FrameParams struct {
	Num      int     `json:"num"`
	KeyCodes []int32 `json:"keycodes"`
	Foods    []int   `json:"foods"`
}

// 专门接收服务器发来的 UDP，通过 chan 转发给 engine
func readUDP(conn net.Conn, chDown chan *comm.Down) {
	defer conn.Close()

	for {
		var buffer [512]byte
		n, err := conn.Read(buffer[:])
		if err != nil {
			astilog.Info("conn.Read:", err)
			return
		}

		down := &comm.Down{}
		err = proto.Unmarshal(buffer[:n], down)
		if err != nil {
			astilog.Info("proto.Unmarshal:", err)
			continue
		}
		chDown <- down
	}
}

func engine() {
	mode := 0
	ticker := time.NewTicker(time.Millisecond * 250)
	var keyCodes []int32
	var myID int32
	var serverConn net.Conn
	chDown := make(chan *comm.Down, 10)

loop:
	for {
		select {
		case m := <-chMode:
			// 重新建立连接
			if serverConn != nil {
				serverConn.Close()
			}
			conn, err := kcp.Dial("127.0.0.1:6688")
			if err != nil {
				astilog.Fatal("udp setup:", err)
			}
			serverConn = conn
			go readUDP(serverConn, chDown)

			if m == 1 {
				// 提交请求启动 1P 模式
				up := &comm.Up{
					M: &comm.Up_Join{
						Join: &comm.Up_UpJoin{
							Mode: comm.Mode_P1P,
						},
					},
				}
				out, _ := proto.Marshal(up)
				n, err := serverConn.Write(out)
				astilog.Debug("serverConn.Write: join:", n, err)
			}

		case down := <-chDown:
			switch cmd := down.M.(type) {
			case *comm.Down_Kickoff:
				astilog.Info("kickoff:", cmd.Kickoff)
				// 启动 1P 模式
				mode = 1
				myID = cmd.Kickoff.Cid
				keyCodes = make([]int32, 1)
				bootstrap.SendMessage(mainWin, "kick-off", cmd.Kickoff)

			case *comm.Down_Frame:
				astilog.Info("frame:", cmd.Frame)
			}

		case kc := <-chKeyCode:
			if serverConn == nil {
				break
			}
			keyCodes[myID] = kc
			// 提交操作按键给服务器
			up := &comm.Up{
				M: &comm.Up_Op{
					Op: &comm.Up_UpOp{
						Keycode: comm.KeyCode(kc),
					},
				},
			}
			out, _ := proto.Marshal(up)
			n, err := serverConn.Write(out)
			astilog.Debug("serverConn.Write: op:", n, err)

		case <-ticker.C:
			if mode <= 0 {
				break
			}
			bootstrap.SendMessage(mainWin, "frame", FrameParams{
				Num:      0,
				KeyCodes: keyCodes,
				Foods:    []int{},
			})
			for id := 0; id < len(keyCodes); id++ {
				keyCodes[id] = 0
			}

		case <-chExit:
			break loop
		}
	}
}
