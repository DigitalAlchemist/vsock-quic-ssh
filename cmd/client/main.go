package main

import (
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/http"
	"os"
	"unicode/utf8"

	utils "digitalalchemy.io/vsock-ssh"
	"github.com/kyokomi/emoji/v2"
	"github.com/lucas-clemente/quic-go"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/mdlayher/vsock"
	"github.com/urfave/cli/v2"
	sshc "golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

func main() {
	app := &cli.App{
		Name: "ssh",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "vsock-enable",
				Value:   false,
				EnvVars: []string{"VSOCK_ENABLE"},
				Hidden:  true,
			},
			&cli.IntFlag{
				Name:    "vsock-cid",
				Value:   1776,
				EnvVars: []string{"VSOCK_CID"},
				Hidden:  true,
			},
		},
		Action: DialSSH,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func DialSSH(ctx *cli.Context) error {

	var rt *http3.RoundTripper = &http3.RoundTripper{
		TLSClientConfig: &tls.Config{
			// TODO: Fix this!!
			InsecureSkipVerify: true,
		},
	}
	if ctx.Bool("vsock-enable") {
		rt.Dial = func(network, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlySession, error) {
			conn, err := vsock.Dial(vsock.Host, uint32(ctx.Int64("vsock-cid")))
			if err != nil {
				return nil, err
			}
			c := &utils.VSockConn{conn}
			return quic.DialEarly(c, nil, "vsock", tlsCfg, cfg)
		}
	}

	vsockHTTPClient := &http.Client{Transport: rt}

	// TODO: NArg
	res, err := vsockHTTPClient.Get("https://127.0.0.1:3000/ssh")
	if err != nil {
		return err
	}
	defer res.Body.Close()

	ds, ok := res.Body.(http3.DataStreamer)
	if !ok {
		return fmt.Errorf("Couldn't convert to DataStreamer")
	}

	stream := ds.DataStream()
	wrap := &utils.QuicStreamConnWrapper{stream}

	// TODO: Add a context
	return DoSSH(wrap)
}

func DoSSH(conn net.Conn) error {
	config := &sshc.ClientConfig{
		User: "test",
		Auth: []sshc.AuthMethod{
			sshc.Password("test"),
		},
		// TODO: Remove this.
		HostKeyCallback: sshc.InsecureIgnoreHostKey(),
	}

	clientConn, chans, reqs, err := sshc.NewClientConn(conn, "conn", config)
	if err != nil {
		return err
	}

	c := sshc.NewClient(clientConn, chans, reqs)

	session, err := c.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	modes := sshc.TerminalModes{
		//sshc.ECHO:          0,     // disable echoing
		//sshc.ICANON:	1,
		//sshc.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
		//sshc.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
	}

	// Request pseudo terminal
	if err := session.RequestPty("xterm", 40, 80, modes); err != nil {
		log.Fatal("request for pseudo terminal failed: ", err)
	}

	em := randomEmojis(5)

	fmt.Fprintf(os.Stderr, "Begin shell on guest - %s\n", em)
	defer func() {
		fmt.Fprintf(os.Stderr, "End shell on guest   - %s\n", em)
	}()

	old, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		log.Fatal("Couldn't set local terminal mode to raw")
	}

	defer term.Restore(int(os.Stdin.Fd()), old)

	if err := session.Run("bash"); err != nil {
		return err
	}

	return nil
}

func randomEmojis(n uint32) string {
	ret := ""
	vals := []string{}
	for _, v := range emoji.CodeMap() {
		if utf8.RuneCountInString(v) > 1 {
			continue
		}
		vals = append(vals, v)
	}

	for i := uint32(0); i < n; i++ {
		v := rand.Intn(len(vals))
		ret += vals[v]
	}
	return ret
}
