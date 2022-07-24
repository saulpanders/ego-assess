package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"time"
)

//icmp报文struct
type icmp struct {
	Type       uint8
	Code       uint8
	Checksum   uint16
	Identifier uint16
	Sequence   uint16
	data       [1024]byte
}

// Ping public method
func Ping(ip string) (bool, error) {
	recv := make([]byte, 1024)                //保存响应数据
	raddr, err := net.ResolveIPAddr("ip", ip) //raddr为目标主机的地址
	if err != nil {
		fmt.Sprintf("resolve ip: %s fail:", ip)
		return false, err
	}
	laddr := net.IPAddr{IP: net.ParseIP("0.0.0.0")} //源地址
	if ip == "" {
		return false, errors.New("ip or domain is null")
	}

	conn, err := net.DialIP("ip4:icmp", &laddr, raddr)
	if err != nil {
		return false, err
	}
	defer conn.Close()

	buffer := assemblyIcmp()
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

/*
求校验和步骤：
（1）把校验和字段置为0；
（2）把需校验的数据看成以16位为单位的数字组成，依次进行二进制反码求和；
（3）把得到的结果存入校验和字段中。
*/
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

func assemblyIcmp() bytes.Buffer {
	var icmpPack icmp
	var buffer bytes.Buffer //数据缓冲

	icmpPack.Type = 8
	icmpPack.Code = 0
	icmpPack.Checksum = 0 //计算Checksum之前置为0
	icmpPack.Identifier = 0
	icmpPack.Sequence = 0
	//copy the data into the buffer
	copy(icmpPack.data[:], "hello")

	//计算校验和
	binary.Write(&buffer, binary.BigEndian, icmpPack) //写入ICMP头
	//binary.Write(&buffer, binary.BigEndian, Data)     //写入自定义数据
	icmpPack.Checksum = checkSum(buffer.Bytes())
	buffer.Reset() //清空buffer

	//生成最终发送数据
	binary.Write(&buffer, binary.BigEndian, icmpPack) //写入ICMP头
	return buffer
}

func main() {
	_, err := Ping(os.Args[1])
	if err != nil {
		fmt.Printf("Error: %s", err)
	} else {
		fmt.Printf("%s Ping ok!", os.Args[1])
	}
}
