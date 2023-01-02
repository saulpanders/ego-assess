/*
	@saulpanders
	client.go (ego-assess)
	inspired by Egress-Assess, multi-protocol DLP test solution

	TODO:
	- FTP Client
	- HTTPS Client
		working https client!
		specify flag toggle for allow insecure?
	- DNSTXT Client
	- ICMP Client

	general refactoring?
	(optional)
	- DNS NS
	- SMTP
*/

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"ego-assess/data"
	//"encoding/base64"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

//icmp stuff

//icmp报文struct
type icmp struct {
	Type       uint8
	Code       uint8
	Checksum   uint16
	Identifier uint16
	Sequence   uint16
	data       [1016]byte
}

func Ping(ip string, data []byte) (bool, error) {
	recv := make([]byte, 1024)
	raddr, err := net.ResolveIPAddr("ip", ip)
	if err != nil {
		fmt.Sprintf("resolve ip: %s fail:", ip)
		return false, err
	}
	laddr := net.IPAddr{IP: net.ParseIP("0.0.0.0")}
	if ip == "" {
		return false, errors.New("ip or domain is null")
	}

	conn, err := net.DialIP("ip4:icmp", &laddr, raddr)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	buffer := assemblyIcmp(data)
	if _, err := conn.Write(buffer.Bytes()); err != nil {
		fmt.Sprintf("post Icmp fail: %v", err)
		return false, err
	}

	conn.SetReadDeadline((time.Now().Add(time.Second * 5)))
	_, err = conn.Read(recv)
	if err != nil {
		fmt.Sprintf("get Icmp fail: %v", err)
		return false, nil
	}

	return true, nil
}

func checkSum(data []byte) uint16 {
	var (
		sum    uint32
		length = len(data)
		index  int
	)
	for length > 1 {
		sum += uint32(data[index])<<8 + uint32(data[index+1])
		index += 2
		length -= 2
	}
	if length > 0 {
		sum += uint32(data[index])
	}
	sum += (sum >> 16)

	return uint16(^sum)
}

func assemblyIcmp(data []byte) bytes.Buffer {
	var icmpPack icmp
	var buffer bytes.Buffer //数据缓冲

	icmpPack.Type = 8
	icmpPack.Code = 0
	icmpPack.Checksum = 0 //计算Checksum之前置为0
	icmpPack.Identifier = 0
	icmpPack.Sequence = 0
	//copy the data into the buffer
	copy(icmpPack.data[:], data)

	//计算校验和
	binary.Write(&buffer, binary.BigEndian, icmpPack) //写入ICMP头
	//binary.Write(&buffer, binary.BigEndian, Data)     //写入自定义数据
	icmpPack.Checksum = checkSum(buffer.Bytes())
	buffer.Reset() //清空buffer

	//生成最终发送数据
	binary.Write(&buffer, binary.BigEndian, icmpPack)
	return buffer
}

/*

refactor into utils later
*/
func getDateTime() string {
	// leave your mark ---> FACTOR THIS INTO SEPARATE FILE?
	//file naming stuff- want it tagged by time
	datetime := strings.ReplaceAll(time.Now().String(), " ", "")
	datetime = strings.ReplaceAll(datetime, ":", "-")
	datetime = string(datetime[:18])
	return datetime
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

	//write remote file
	datetime := getDateTime()
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
	//Read the response body --> not strictly necessary, wousld prob suffice to check for 200
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	sb := string(body)
	log.Printf(sb)
}

//sets up the necessary things to transmit HTTP data
func (client Client) transmitHTTPS() {
	url := "https://" + client.Target + ":" + client.Port + "/secure"

	//allow insecure HTTPS for now
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	https := &http.Client{Transport: tr}

	//Leverage Go's HTTP Post function to make request
	resp, err := https.Post(url, "application/json", client.Data)

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

//sets up the necessary things to transmit ICMP data
//WORKING SEND/RECIEVE ICMP!!
//add logic for base64 encoding and signaliing server to start transfer...
func (client Client) transmitICMP() {
	totalPackets := 0
	bytesRead := 0
	var icmpSample icmp

	//icmp starttransmit signature
	var transmitHeader = ".:::-989-:::."

	hbytes := bytes.NewBuffer([]byte(transmitHeader))

	//hbytes.ReadFrom(client.Data)

	client.Data = hbytes
	full := client.Data.Len()
	//.calcalate total packets
	if (client.Data.Len() % len(icmpSample.data)) == 0 {
		totalPackets = (client.Data.Len()) / (len(icmpSample.data))
	} else {
		totalPackets = (client.Data.Len())/(len(icmpSample.data)) + 1
	}

	for {
		if !(bytesRead < full) {
			break
		}

		_, err := Ping(client.Target, client.Data.Next(len(icmpSample.data)))
		if err != nil {
			fmt.Printf("Error: %s", err)
		}
		bytesRead += len(icmpSample.data)

	}
	fmt.Println("Done pinging!, used %d packets\n", totalPackets)

}

//Interface function - switches between client types
func (client Client) transmit() {
	switch client.Protocol {
	case "HTTP":
		client.transmitHTTP()
	case "HTTPS":
		client.transmitHTTPS()
	case "ICMP":
		client.transmitICMP()
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
	flag.StringVar(&exfilProtocol, "type", "HTTP", "protocol type for exfil (HTTP/HTTPS/SFTP)")
	flag.StringVar(&remoteHost, "target", "127.0.0.1", "target host/exfil server")
	flag.StringVar(&remotePort, "port", "8080", "remote server exfil port")
	flag.StringVar(&username, "user", "testuser", "username (for SSH/SFTP)")
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
	//add base64 serialization here?
	client = Client{remoteHost, remotePort, exfilProtocol, username, password, contents}
	client.transmit()

}
