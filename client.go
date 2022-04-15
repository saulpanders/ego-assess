/*
	@saulpanders
	client.go (ego-assess)
	inspired by Egress-Assess, multi-protocol DLP test solution

	implement HTTP version first (easiest)

	BASIC CLIENT WORKS!

	TODO: Client side command line arg logic
*/

package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"path/filepath"
	"strings"
	//"encoding/json"
	"ego-assess/data"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

type Sender interface {
	transmit()
}

type Client struct {
	Target   string
	Port     string
	Protocol string

	Username string
	Password string
	Data     *bytes.Buffer
}

//ripped this from somewhere: checks known_hosts for current user to use with SSH config. May be unneccessary

func checkHostKey(host, port string) (ssh.PublicKey, error) {
	// $HOME/.ssh/known_hosts
	homedir, _ := os.UserHomeDir()
	file, err := os.Open(filepath.Join(homedir, ".ssh", "known_hosts"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var hostport string
	if port == "22" {
		// standard port assumes 22
		// 192.168.10.53 ssh-rsa AAAAB3Nza...vguvx+81N1xaw==
		hostport = host
	} else {
		// non-standard port(s)
		// [ssh.example.com]:1999,[93.184.216.34]:1999 ssh-rsa AAAAB3Nza...vguvx+81N1xaw==
		hostport = "[" + host + "]:" + port
	}

	scanner := bufio.NewScanner(file)
	var hostKey ssh.PublicKey
	for scanner.Scan() {
		fields := strings.Split(scanner.Text(), " ")
		if len(fields) != 3 {
			continue
		}
		if strings.Contains(fields[0], hostport) {
			var err error
			hostKey, _, _, _, err = ssh.ParseAuthorizedKey(scanner.Bytes())
			if err != nil {
				return nil, fmt.Errorf("Error parsing %q: %v", fields[2], err)
			}
			break // scanning line by line, first occurrence will be returned
		}
	}
	if hostKey == nil {
		return nil, fmt.Errorf("No hostkey for %s", host+":"+port)
	}
	return hostKey, nil
}

//workign SFTP client: connects to server and writes remote file
func (client Client) transmitSFTP() {
	hostKey, err := checkHostKey(client.Target, client.Port)
	if err != nil {
		log.Fatal(err)
	}

	config := &ssh.ClientConfig{
		User: client.Username,
		Auth: []ssh.AuthMethod{
			ssh.Password(client.Password),
		},
		HostKeyCallback: ssh.FixedHostKey(hostKey), //could make this IngoreInsecureHostKye?
	}
	conn, err := ssh.Dial("tcp", client.Target+":"+client.Port, config)
	if err != nil {
		log.Fatal("Failed to dial: ", err)
	}
	defer conn.Close()

	// open an SFTP session over an existing sshz connection.
	sftpclient, err := sftp.NewClient(conn)
	if err != nil {
		log.Fatal(err)
	}
	defer sftpclient.Close()

	// leave your mark ---> FACTOR THIS INTO SEPARATE FILE?
	//file naming stuff- want it tagged by time
	datetime := strings.ReplaceAll(time.Now().String(), " ", "")
	datetime = strings.ReplaceAll(datetime, ":", "-")
	datetime = datetime[:18]

	filename := datetime + "-remote-data.txt"

	f, err := sftpclient.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	// create new buffer to write exfil --> possibly break into util function?
	buffer := bufio.NewWriter(f)

	_, err = buffer.Write(client.Data.Bytes())
	if err != nil {
		log.Fatal(err)
	}
	if err := buffer.Flush(); err != nil {
		log.Fatal(err)
	}

}

//sets up the necessary things to transmit HTTP data
func (client Client) transmitHTTP() {
	url := "http://" + client.Target + ":" + client.Port + "/"

	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post(url, "application/json", client.Data)

	//Handle Error
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()
	//Read the response body --> not strictly necessary, would prob suffice to check for 200
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	sb := string(body)
	log.Printf(sb)
}

//Interface function - switches between client types
func (client Client) transmit() {
	switch client.Protocol {
	case "HTTP":
		client.transmitHTTP()
	case "SFTP":
		client.transmitSFTP()
	default:
		log.Fatal("[x] please choose a supported client protocol")
	}
}

//main function
func main() {
	var client Sender

	var (
		remoteHost    string
		remotePort    string
		exfilProtocol string
		username      string
		password      string

		//future data args
		dataType string
		dataSize int
	)

	//should prob make this by protocol, not port but ehhh
	flag.StringVar(&exfilProtocol, "protocol", "HTTP", "protocol for exfil")
	flag.StringVar(&remoteHost, "target", "127.0.0.1", "target host/exfil server")
	flag.StringVar(&remotePort, "port", "8080", "remote server exfil port")
	flag.StringVar(&username, "user", "testuser", "username (for SSH/SFTP")
	flag.StringVar(&password, "password", "test1234", "password (for SSH/SFTP)")
	flag.StringVar(&dataType, "datatype", "ssn", "type of data for exfil (SSN or CC)")
	flag.IntVar(&dataSize, "size", 10, "size of data file for exfil (in MB)")
	flag.Parse()

	filename := data.CreateDataFile(dataType, dataSize)

	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("Error while opening file. Err: %s", err)
	}
	defer file.Close()

	fileBuffer := new(bytes.Buffer)
	fileBuffer.ReadFrom(file)
	contents := bytes.NewBuffer(fileBuffer.Bytes())
	client = Client{remoteHost, remotePort, exfilProtocol, username, password, contents}
	client.transmit()

}
