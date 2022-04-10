/*
	@saulpanders
	server.go (ego-assess)
	inspired by Egress-Assess, multi-protocol DLP test solution

	Implement HTTP version first

	https://shareablecode.com/snippets/golang-http-server-post-request-example-how-handle-payload-body-form-data-go-AuUN-ztyc
	https://golangbyexample.com/basic-http-server-go/

	BASIC SERVER WORKING! Can listen and respond to GET/POST requests

	TODO: Basic command-line arg parsing
*/

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

//practicing implementing these as interfaces so it will be easier to "lift and shift code"

type Receiver interface {
	serve()
}

//struct to implement the interface
type HttpServer struct {
	Protocol string
	Port     string
}

//Interface function - sets up the necessary things to serve HTTP
func (server HttpServer) serve() {
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
	switch r.Method {
	case "GET":
		fmt.Fprintf(w, "This is GET request at path = %s", r.URL.Path)
	case "POST":
		//possibly add base64 deserialization here
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			log.Fatal("[-] Error parsing request body: ", err)
		}
		fmt.Fprintf(w, "POST req made at %s with body %s", r.URL.Path, data)
	default:
		fmt.Fprintf(w, "Request method %s is not supported", r.Method)
	}
}

//main function
func main() {
	var server Receiver
	server = HttpServer{"HTTP", "8080"}

	server.serve()

}
