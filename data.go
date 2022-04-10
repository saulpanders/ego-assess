/*
	@saulpanders
	data.go (ego-assess)
	inspired by Egress-Assess, multi-protocol DLP test solution

	TODO: Client side logic for generative credit cards

	WORKING SSN GENERATOR!
*/

package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type data interface {
	generate_data() string
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

func buildSSNs(datasize int) string {
	var ssns string
	rand.Seed(time.Now().UnixNano())

	//approx 1 meg of data is 81500* datasize
	for i := 0; i < (datasize); i++ {
		ssns += createSSN() + ", "
	}
	return ssns
}

func (datafile ExfilData) generate_data() string {
	fmt.Printf("[+] Generating %s data...\n", datafile.DataType)
	var data string

	switch datafile.DataType {
	case "ssn":
		data = buildSSNs(datafile.DataSize)

	default:
		fmt.Println("[-] Error, something went wrong...")
	}
	return data
}

//main function
func main() {
	var datafile data

	datafile = ExfilData{"ssn", 10, "Fake SSNs", "text"}

	f, err := os.Create("data.txt")

	if err != nil {
		log.Fatal(err)
	}

	defer f.Close()

	_, err2 := f.WriteString(datafile.generate_data())

	if err2 != nil {
		log.Fatal(err2)
	}

	fmt.Println("[+] done!")

}
