package test

import (
	"context"
	"encoding/json"
	"pzrp/client"
	"pzrp/pkg/config"
	"pzrp/pkg/utils"
	"pzrp/server"
	"testing"
	"time"
)

func loadConfig() (*config.ServerConf, *config.ClientConf) {
	confStr := `{
		"server_addr": "127.0.0.1",
		"server_port": 8848,
		"services": {
			"s1": {
				"type": "tcp",
				"local_ip": "127.0.0.1",
				"local_port": "8100-8105",
				"remote_port": "9200-9205"
			},
			"s2": {
				"type": "udp",
				"local_ip": "127.0.0.1",
				"local_port": "8500,8600,8700",
				"remote_port": "9500,9600,9700"
			}
		}
	}`
	var c config.ClientConf
	err := json.Unmarshal([]byte(confStr), &c)
	if err != nil {
		panic(err)
	}
	confStr = `{
		"bind_addr": "127.0.0.1",
		"bind_port": 8848
	}`
	var s config.ServerConf
	err = json.Unmarshal([]byte(confStr), &s)
	if err != nil {
		panic(err)
	}
	return &s, &c
}

func TestTCP(t *testing.T) {
	conf1, conf2 := loadConfig()
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer func() {
		cancel1()
		cancel2()
	}()

	go func() {
		server.Run(ctx1, conf1)
	}()
	go func() {
		client.Run(ctx2, conf2)
	}()
	buf := make([]byte, 1024)
	// test sending and receiving data
	con1, con2 := utils.TCPipe(8100, 9200, 100*time.Millisecond)
	msg1 := "hello"
	con1.Write([]byte(msg1))
	n, err := con2.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if msg1 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg1, string(buf[:n]))
	}
	msg2 := "world"
	con2.Write([]byte(msg2))
	n, err = con1.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if msg2 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg2, string(buf[:n]))
	}

	msg3 := "hi!"
	con2.Write([]byte(msg3))
	n, err = con1.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if msg3 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg3, string(buf[:n]))
	}
	msg4 := "bye~"
	con1.Write([]byte(msg4))
	n, err = con2.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if msg4 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg4, string(buf[:n]))
	}
	// Test server actively shuts down
	con1.Close()
	n, _ = con2.Read(buf)
	if n != 0 {
		t.Fatal("not closing normally")
	}
	// Test client actively shuts down
	con1, con2 = utils.TCPipe(8101, 9201, 100*time.Millisecond)
	con2.Close()
	n, _ = con1.Read(buf)
	if n != 0 {
		t.Fatal("not closing normally")
	}
	// Test half closed
	con1, con2 = utils.TCPipe(8102, 9202, 100*time.Millisecond)
	con1.CloseWrite()
	con2.Write([]byte(msg1))

	n, err = con1.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if msg1 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg1, string(buf[:n]))
	}

	con2.Write([]byte(msg2))
	n, err = con1.Read(buf)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if msg2 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg2, string(buf[:n]))
	}

	con2.CloseWrite()

	n, _ = con1.Read(buf)
	if n != 0 {
		t.Fatal("not closing normally")
	}
	n, _ = con2.Read(buf)
	if n != 0 {
		t.Fatal("not closing normally")
	}
}

func TestUDP(t *testing.T) {
	conf1, conf2 := loadConfig()
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer func() {
		cancel1()
		cancel2()
	}()

	go func() {
		server.Run(ctx1, conf1)
	}()
	go func() {
		client.Run(ctx2, conf2)
	}()
	buf := make([]byte, 1024)
	con1, con2 := utils.UDPipe(8500, 9500, 100*time.Millisecond)

	msg1 := "Hello!"
	_, err := con2.Write([]byte(msg1))
	if err != nil {
		t.Fatalf("udp write error: %v", err)
	}

	n, addr, err := con1.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("udp read error: %v", err)
	}
	if msg1 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg1, string(buf[:n]))
	}

	msg2 := "Hi~"
	_, err = con1.WriteTo([]byte(msg2), addr)
	if err != nil {
		t.Fatalf("udp write error: %v", err)
	}

	n, err = con2.Read(buf)
	if err != nil {
		t.Fatalf("udp read error: %v", err)
	}
	if msg2 != string(buf[:n]) {
		t.Fatalf("inconsistent data: expect %v, but got %v", msg2, string(buf[:n]))
	}

}
