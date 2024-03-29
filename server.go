/*
	@saulpanders
	.go (ego-assess)
	inspired by Egress-Assess, multi-protocol DLP test solution


	https://shareablecode.com/snippets/golang-http-server-post-request-example-how-handle-payload-body-form-data-go-AuUN-ztyc
	https://golangbyexample.com/basic-http-server-go/

	TODO:
	- FTP Server
		ideas: https://betterprogramming.pub/how-to-write-a-concurrent-ftp-server-in-go-part-1-3904f2e3a9e5
	- HTTPS Server
		working HTTPS server!
		add flags for cert&key specification
		port gencert.go into something else
		add letsencrypt support? (autocert)
	- DNSTXT Server
	- ICMP Server
			mostly done, just need bug testing & outfile writing

	general refactoring?
	(optional)
	- DNS NS
	- SMTP
*/

package main

import (
	"bufio"
	"bytes"
	"ego-assess/data"
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/icmp"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

/*
	refactor this to utils later?

*/
func getDateTime() string {
	//file naming stuff- want it tagged by time
	datetime := strings.ReplaceAll(time.Now().String(), " ", "")
	datetime = strings.ReplaceAll(datetime, ":", "-")
	datetime = string(datetime[:18])
	return datetime
}

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
	IP       string
}

func writeOufile(filename, text) {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.WriteString(text); err != nil {
		panic(err)
	}
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

//Struct method - sets up the necessary things to serve HTTPS
//using gencert.go for self-sgned cert at this point.. add letsencrypt support?
// disable SSL cecks in powershel? [System.Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }
func (server Server) serveHTTPS() {
	//Create the default mux
	mux := http.NewServeMux()

	mux.HandleFunc("/secure", httpHandler)

	//Create the server.
	serverInstance := &http.Server{
		Addr:    ":" + server.Port,
		Handler: mux,
	}

	data.GenerateSelfSignedCert()

	err := serverInstance.ListenAndServeTLS("server.crt", "server.key")
	if err != nil {
		log.Fatal("[-] HTTPS Server Error: ", err)
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

		datetime := getDateTime()
		filename := datetime + "-remote-data.txt"
		f, err := os.Create(filename)
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

//ripped this mostly from the standard package examples
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

//should technically be called ListenICMP but ehhh
//testing icmp listening functionality
//WORKING SEND/RECIEVE ICMP!!
//add logic for base64 encoding and signaliing server to start transfer...
func (server Server) serveICMP() {
	conn, err := icmp.ListenPacket("ip4:icmp", server.IP)
	clientIP := "0.0.0.0"
	filename := "temp"
	if err != nil {
		log.Fatal(err)
	}

	for {
		msg := make([]byte, 1500)
		length, sourceIP, err := conn.ReadFrom(msg)
		if err != nil {
			log.Println(err)
			continue
		}

		blob := string(msg[8:])
		decodedMessage, _ := base64.StdEncoding.DecodeString(blob)

		if sourceIP.String() == clientIP {
			log.Printf("message = '%s', length = %d, source-ip = %s", string(decodedMessage), length, sourceIP)
			writeOufile(filename, decodedMessage)

		}
		if bytes.Contains(decodedMessage, []byte(".:::-989-:::.")) {
			log.Printf("message = '%s', length = %d, source-ip = %s", string(decodedMessage), length, sourceIP)
			clientIP = sourceIP.String()

			//create outfile in this if statement since this should be the start of the transmission. Afer that we use IP to track the client
			datetime := getDateTime()
			filename = datetime + "-remote-data.txt"
			writeOufile(filename, decodedMessage)
			

	}
	_ = conn.Close()

}

//Interface function - switches between server struct implementations
func (server Server) serve() {
	switch server.Protocol {
	case "HTTP":
		server.serveHTTP()
	case "HTTPS":
		server.serveHTTPS()
	case "SFTP":
		server.serveSFTP()
	case "ICMP":
		server.serveICMP()
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
		serverIP   string
	)

	/*
		add option to specifcy SSL certs, otherwise it will generate the self-sgined one

	*/

	//flag.BoolVar(&debugStderr, "e", false, "debug to stderr")
	flag.StringVar(&serverType, "type", "HTTP", "server protocol (HTTP/HTTPS/SFTP/DNS/ICMP)")
	flag.StringVar(&serverPort, "port", "1234", "server port")
	flag.StringVar(&serverUser, "user", "testuser", "username (for SSH/SFTP")
	flag.StringVar(&serverPass, "password", "test1234", "password (for SSH/SFTP)")
	flag.StringVar(&serverIP, "ip", "127.0.0.1", "IP address to listen on (for ICMP, optional)")
	flag.Parse()

	//debugStream := ioutil.Discard
	//if debugStderr {
	//	debugStream = os.Stderr
	//}

	server = Server{serverType, serverPort, serverUser, serverPass, serverIP}
	server.serve()
}
