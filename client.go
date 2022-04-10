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
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

type Sender interface {
	transmit()
}

type HttpClient struct {
	Target string
	Port   string
	Data   *bytes.Buffer
}

//Interface function - sets up the necessary things to transmit HTTP data
func (client HttpClient) transmit() {
	url := "http://" + client.Target + ":" + client.Port + "/"

	//Leverage Go's HTTP Post function to make request
	resp, err := http.Post(url, "application/json", client.Data)

	//Handle Error
	if err != nil {
		log.Fatalf("An Error Occured %v", err)
	}
	defer resp.Body.Close()
	//Read the response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	sb := string(body)
	log.Printf(sb)
}

//main function
func main() {
	var client Sender

	//using JSON as test data for now
	postBody, _ := json.Marshal(map[string]string{
		"name":  "Toby",
		"email": "Toby@example.com",
	})
	responseBody := bytes.NewBuffer(postBody)

	client = HttpClient{"127.0.0.1", "8080", responseBody}
	client.transmit()

}
