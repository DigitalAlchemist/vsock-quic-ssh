package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"os"

	utils "digitalalchemy.io/vsock-ssh"
	"github.com/lucas-clemente/quic-go/http3"
	"github.com/mdlayher/vsock"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name: "agentd",
		Flags: []cli.Flag{
			// vSock-specific parameters
			&cli.BoolFlag{
				Name:    "vsock-enable",
				Value:   false,
				EnvVars: []string{"VSOCK_ENABLE"},
			},
			&cli.IntFlag{
				Name:    "vsock-cid",
				Value:   1776,
				EnvVars: []string{"VSOCK_CID"},
			},
			// Unix Socket Parameters
			&cli.BoolFlag{
				Name:    "unix-enable",
				Value:   false,
				EnvVars: []string{"UNIX_ENABLE"},
			},
			&cli.StringFlag{
				Name:    "unix-path",
				Value:   "/tmp/vsock-ssh.sock",
				EnvVars: []string{"UNIX_PATH"},
			},
			// UDP-specific parameters
			&cli.BoolFlag{
				Name:    "udp-enable",
				Value:   false,
				EnvVars: []string{"UDP_ENABLE"},
			},
			&cli.IntFlag{
				Name:    "udp-port",
				Value:   8443,
				EnvVars: []string{"UDP_PORT"},
			},
			// QUIC-specific parameters
			&cli.StringFlag{
				Name:    "quic-cert",
				Value:   "cert.pem",
				EnvVars: []string{"QUIC_CERT"},
			},
			&cli.StringFlag{
				Name:    "quic-key",
				Value:   "key.pem",
				EnvVars: []string{"QUIC_KEY"},
			},
		},
		Action: ListenApp,
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func ListenApp(ctx *cli.Context) error {

	// TODO: Register multiple handlers -- only the vsock should have the SSH proto.
	// Setup regular handler
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		data, _ := httputil.DumpRequest(req, true)
		fmt.Fprintf(w, "%s", string(data))
		return
	})

	// Setup our hijackable handler for SSH connections
	http.HandleFunc("/ssh", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		ds, ok := w.(http3.DataStreamer)
		if !ok {
			http.Error(w, "Invalid Protocol", http.StatusUpgradeRequired)
			return
		}

		stream := ds.DataStream()
		wrap := &utils.QuicStreamConnWrapper{Stream: stream}
		utils.SSHHandleConn(wrap)
		return
	})

	closer := make(chan error)
	if ctx.Bool("vsock-enable") {
		log.Println("Listening on vsock")
		go func() {
			closer <- ListenAndServeVSock(uint32(ctx.Int("vsock-cid")), ctx.String("quic-cert"), ctx.String("quic-key"), nil)
		}()
	}
	if ctx.Bool("udp-enable") {
		go func() {
			log.Println("Starting quic listener on :3000")
			closer <- http3.ListenAndServeQUIC(":3000", ctx.String("quic-cert"), ctx.String("quic-key"), nil)
		}()
		go func() {
			log.Println("Starting HTTPS listener on :3000")
			closer <- http.ListenAndServeTLS(":3000", ctx.String("quic-cert"), ctx.String("quic-key"), nil)
		}()
	}

	if ctx.Bool("unix-enable") {
		// TODO
	}

	err := <-closer
	return err
}

func ListenAndServeVSock(vsockCid uint32, cert, key string, handler http.Handler) error {

	var err error
	certs := make([]tls.Certificate, 1)

	certs[0], err = tls.LoadX509KeyPair(cert, key)
	if err != nil {
		return err
	}

	config := &tls.Config{
		Certificates: certs,
	}

	httpServer := &http.Server{
		Addr:      fmt.Sprintf("vsock:%d", vsockCid),
		TLSConfig: config,
	}

	quicServer := &http3.Server{
		Server: httpServer,
	}

	if handler == nil {
		handler = http.DefaultServeMux
	}

	httpServer.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quicServer.SetQuicHeaders(w.Header())
		handler.ServeHTTP(w, r)
	})

	// TODO: Get from context
	listener, err := vsock.Listen(uint32(vsockCid))
	if err != nil {
		return err
	}
	defer listener.Close()

	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}

		log.Printf("VSock Connection from %s\n", conn.RemoteAddr().String())

		err = quicServer.Serve(&utils.VSockConn{Conn: conn})
		if err != nil && err != io.EOF {
			log.Println(err)
			return err
		}
	}

	return nil

}
