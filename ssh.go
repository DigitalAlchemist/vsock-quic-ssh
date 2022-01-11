package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/creack/pty"
	"github.com/gliderlabs/ssh"
	sshc "golang.org/x/crypto/ssh"
)

func init() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}
	signer, err := sshc.NewSignerFromKey(key)
	if err != nil {
		log.Fatal(err)
	}
	sshServer.HostSigners = []ssh.Signer{
		signer,
	}

	if sshServer.RequestHandlers == nil {
		sshServer.RequestHandlers = map[string]ssh.RequestHandler{}
		for k, v := range ssh.DefaultRequestHandlers {
			sshServer.RequestHandlers[k] = v
		}
	}

	if sshServer.ChannelHandlers == nil {
		sshServer.ChannelHandlers = map[string]ssh.ChannelHandler{}
		for k, v := range ssh.DefaultChannelHandlers {
			sshServer.ChannelHandlers[k] = v
		}
	}
	if sshServer.SubsystemHandlers == nil {
		sshServer.SubsystemHandlers = map[string]ssh.SubsystemHandler{}
		for k, v := range ssh.DefaultSubsystemHandlers {
			sshServer.SubsystemHandlers[k] = v
		}
	}

}

func setWinsize(f *os.File, w, h int) {
	syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), uintptr(syscall.TIOCSWINSZ),
		uintptr(unsafe.Pointer(&struct{ h, w, x, y uint16 }{uint16(h), uint16(w), 0, 0})))
}

var sshServer *ssh.Server = &ssh.Server{
	ConnectionFailedCallback: func(conn net.Conn, err error) {
		log.Println(err)
	},

	Handler: func(s ssh.Session) {
		log.Printf("Handling connection from %s\n", s.RemoteAddr().String())
		cmd := exec.Command("bash")
		ptyReq, winCh, isPty := s.Pty()
		//fmt.Printf("IsPty? %v\n", isPty)
		if isPty {
			cmd.Env = append(cmd.Env, fmt.Sprintf("TERM=%s", ptyReq.Term))
			f, err := pty.Start(cmd)
			if err != nil && err != io.EOF {
				fmt.Printf("Received a fatal error: %s", err)
				return
			}
			go func() {
				for win := range winCh {
					setWinsize(f, win.Width, win.Height)
				}
			}()
			go func() {
				_, err = io.Copy(f, s) // stdin
				if err != nil && err != io.EOF {
					fmt.Printf("Received a fatal error: %s", err)
					return
				}
			}()
			_, err = io.Copy(s, f) // stdout
			if err != nil && err != io.EOF {
				if _, ok := err.(*fs.PathError); !ok {
					fmt.Printf("Received a fatal error: %#v", err)
					return
				}
			}
			cmd.Wait()
		} else {
			io.WriteString(s, "No pty")
			return
		}
	},
}

func SSHHandleConn(conn net.Conn) error {
	// TODO: Fix this.
	sshServer.HandleConn(conn)
	return nil
}
