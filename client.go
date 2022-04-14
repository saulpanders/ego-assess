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
	"bytes"
	"flag"
	//"encoding/json"
	"ego-assess/data"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

type Sender interface {
	transmit()
}

type Client struct {
	Target   string
	Port     string
	Protocol string
	Data     *bytes.Buffer
}

//Interface function - sets up the necessary things to transmit HTTP data
func (client Client) transmitHTTP() {
	url := "http://" + client.Target + ":" + client.Port + "/"

	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post(url, "application/json", client.Data)

	//Handle Error
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()
	//Read the response body --> not strictly necessary, would prob suffice to 200
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	sb := string(body)
	log.Printf(sb)
}

//Interface function - sets up the necessary things to transmit HTTP data
func (client Client) transmit() {
	switch client.Protocol {
	case "HTTP":
		client.transmitHTTP()
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
	client = Client{remoteHost, remotePort, exfilProtocol, contents}
	client.transmit()

}
