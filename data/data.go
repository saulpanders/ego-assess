/*
	@saulpanders
	data.go (ego-assess)
	inspired by Egress-Assess, multi-protocol DLP test solution

	TODO: Client side logic for generative credit cards
*/

//package main
package data

import (
	"bufio"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"
)

type data interface {
	generate_data() []string
}

type ExfilData struct {
	DataType    string
	DataSize    int //in MB
	Description string
	Filetype    string
}

func createSSN() string {
	//SSNs are 9 digits long
	digitString := strconv.Itoa(rand.Intn(999999999-100000000) + 100000000)

	ssn := digitString[0:3] + "-" + digitString[3:5] + "-" + digitString[5:9]
	return ssn
}

func buildSSNs(datasize int) []string {
	var ssns []string
	rand.Seed(time.Now().UnixNano())

	//approx 1 meg of data is 81500* datasize
	for i := 0; i < (datasize * 81500); i++ {
		ssns = append(ssns, createSSN())
	}
	return ssns
}

func (datafile ExfilData) generate_data() []string {
	fmt.Printf("[+] Generating %s data...\n", datafile.DataType)
	var data []string

	switch datafile.DataType {
	case "ssn":
		data = buildSSNs(datafile.DataSize)

	default:
		fmt.Println("[-] Error, something went wrong...")
	}
	return data
}

func getDateTime() string {
	//file naming stuff- want it tagged by time
	datetime := strings.ReplaceAll(time.Now().String(), " ", "")
	datetime = strings.ReplaceAll(datetime, ":", "-")
	datetime = string(datetime[:18])
	return datetime
}

//exported function -- called from Client
func CreateDataFile(dataType string, dataSize int) string {
	var datafile data

	//fix hardcode later
	datafile = ExfilData{dataType, dataSize, "Fake SSNs", "text"}

	datetime := getDateTime()

	filename := datetime + "-local-data.txt"
	f, err := os.Create(filename)

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	// create new buffer for writing data to file
	buffer := bufio.NewWriter(f)

	for _, line := range datafile.generate_data() {
		_, err := buffer.WriteString(line + "\n")
		if err != nil {
			log.Fatal(err)
		}
	}

	// flush buffered data to the file
	if err := buffer.Flush(); err != nil {
		log.Fatal(err)
	}

	fmt.Println("[+] done!")

	//return filename so we can read it in client file
	return filename

}
