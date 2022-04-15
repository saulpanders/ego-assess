/*
	@saulpanders
	.go (ego-assess)
	inspired by Egress-Assess, multi-protocol DLP test solution

	Implement HTTP version first

	https://shareablecode.com/snippets/golang-http-server-post-request-example-how-handle-payload-body-form-data-go-AuUN-ztyc
	https://golangbyexample.com/basic-http-server-go/

	BASIC SERVER WORKING! Can listen and respond to GET/POST requests

	TODO: Basic command-line arg parsing
*/

package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

//practicing writing interfaces
type Receiver interface {
	serve()
}

//struct to implement the interface
type Server struct {
	Protocol string
	Port     string

	//for SSH/SFTP
	Username string
	Password string
}

//Struct method - sets up the necessary things to serve HTTP
func (server Server) serveHTTP() {
	//Create the default mux
	mux := http.NewServeMux()

	//Handling the /. The handler is a function here (may need another for /post_data)
	mux.HandleFunc("/", httpHandler)

	//Create the server.
	serverInstance := &http.Server{
		Addr:    ":" + server.Port,
		Handler: mux,
	}

	err := serverInstance.ListenAndServe()
	if err != nil {
		log.Fatal("[-] HTTP Server Error: ", err)
	}
}

//HTTP Handler, used to parse response from client
func httpHandler(w http.ResponseWriter, r *http.Request) {
	//possibly add special header check here?
	switch r.Method {
	case "GET":
		fmt.Fprintf(w, "[!] This is GET request at path = %s", r.URL.Path)
	case "POST":
		//possibly add base64 deserialization here
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatal("[-] Error parsing request body: ", err)
		}

		//write responde body
		fmt.Fprintf(w, "POST req made at %s from %s", r.URL.Path, r.RemoteAddr)
		//log info server-side
		fmt.Printf("[+] POST req made at %s from %s\n", r.URL.Path, r.RemoteAddr)
		fmt.Println("[+] writing exfil to file..")
		//write exfil data to file
		f, err := os.Create("exfil.txt")
		if err != nil {
			log.Fatal(err)
		}

		defer f.Close()

		// create new buffer to write exfil --> possibly break into util function?
		buffer := bufio.NewWriter(f)

		_, err = buffer.Write(data)
		if err != nil {
			log.Fatal(err)
		}
		if err := buffer.Flush(); err != nil {
			log.Fatal(err)
		}

		fmt.Println("[+] exfil done!")

	default:
		fmt.Fprintf(w, "Request method %s is not supported", r.Method)
	}
}

func (server Server) serveSFTP() {
	//hard coding this for tesitng purposes
	debugStream := os.Stderr

	// An SSH server is represented by a ServerConfig, which holds
	// certificate details and handles authentication of ServerConns.
	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			// Should use constant-time compare (or better, salt+hash) in
			// a production setting.
			fmt.Fprintf(debugStream, "Login: %s\n", c.User())
			//specify this from command line args
			if c.User() == server.Username && string(pass[:]) == server.Password {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}

	privateBytes, err := ioutil.ReadFile("id_rsa")
	if err != nil {
		log.Fatal("Failed to load private key", err)
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		log.Fatal("Failed to parse private key", err)
	}

	config.AddHostKey(private)

	// Once a ServerConfig has been configured, connections can be
	// accepted.

	//specify port from server object
	listener, err := net.Listen("tcp", "0.0.0.0:"+server.Port)
	if err != nil {
		log.Fatal("failed to listen for connection", err)
	}
	fmt.Printf("Listening on %v\n", listener.Addr())

	nConn, err := listener.Accept()
	if err != nil {
		log.Fatal("failed to accept incoming connection", err)
	}

	// Before use, a handshake must be performed on the incoming
	// net.Conn.
	_, chans, reqs, err := ssh.NewServerConn(nConn, config)
	if err != nil {
		log.Fatal("failed to handshake", err)
	}
	fmt.Fprintf(debugStream, "SSH server established\n")

	// The incoming Request channel must be serviced.
	go ssh.DiscardRequests(reqs)

	// Service the incoming Channel channel.
	for newChannel := range chans {
		// Channels have a type, depending on the application level
		// protocol intended. In the case of an SFTP session, this is "subsystem"
		// with a payload string of "<length=4>sftp"
		fmt.Fprintf(debugStream, "Incoming channel: %s\n", newChannel.ChannelType())
		if newChannel.ChannelType() != "session" {
			newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
			fmt.Fprintf(debugStream, "Unknown channel type: %s\n", newChannel.ChannelType())
			continue
		}
		channel, requests, err := newChannel.Accept()
		if err != nil {
			log.Fatal("could not accept channel.", err)
		}
		fmt.Fprintf(debugStream, "Channel accepted\n")

		// Sessions have out-of-band requests such as "shell",
		// "pty-req" and "env".  Here we handle only the
		// "subsystem" request.
		go func(in <-chan *ssh.Request) {
			for req := range in {
				fmt.Fprintf(debugStream, "Request: %v\n", req.Type)
				ok := false
				switch req.Type {
				case "subsystem":
					fmt.Fprintf(debugStream, "Subsystem: %s\n", req.Payload[4:])
					if string(req.Payload[4:]) == "sftp" {
						ok = true
					}
				}
				fmt.Fprintf(debugStream, " - accepted: %v\n", ok)
				req.Reply(ok, nil)
			}
		}(requests)

		serverOptions := []sftp.ServerOption{
			sftp.WithDebug(debugStream),
		}

		//read-only toggle, removed for now
		/*
			if readOnly {
				serverOptions = append(serverOptions, sftp.ReadOnly())
				fmt.Fprintf(debugStream, "Read-only server\n")
			} else {
				fmt.Fprintf(debugStream, "Read write server\n")
			}
		*/

		server, err := sftp.NewServer(
			channel,
			serverOptions...,
		)
		if err != nil {
			log.Fatal(err)
		}
		if err := server.Serve(); err == io.EOF {
			server.Close()
			log.Print("sftp client exited session.")
		} else if err != nil {
			log.Fatal("sftp server completed with error:", err)
		}
	}
}

//Interface function - switches between server struct implementations
func (server Server) serve() {
	switch server.Protocol {
	case "HTTP":
		server.serveHTTP()
	case "SFTP":
		server.serveSFTP()
	default:
		log.Fatal("[x] please choose a supported server protocol")
	}

}

//main function
func main() {
	var server Receiver
	var (
		//debugStderr bool
		serverType string
		serverPort string
		serverUser string
		serverPass string
	)

	//flag.BoolVar(&debugStderr, "e", false, "debug to stderr")
	flag.StringVar(&serverType, "type", "HTTP", "server protocol (HTTP/HTTPS/SFTP/DNS/ICMP)")
	flag.StringVar(&serverPort, "port", "1234", "server port")
	flag.StringVar(&serverUser, "user", "testuser", "username (for SSH/SFTP")
	flag.StringVar(&serverPass, "password", "test1234", "password (for SSH/SFTP)")
	flag.Parse()

	//debugStream := ioutil.Discard
	//if debugStderr {
	//	debugStream = os.Stderr
	//}

	server = Server{serverType, serverPort, serverUser, serverPass}
	server.serve()
}
