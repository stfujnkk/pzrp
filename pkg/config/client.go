package config

import (
	"encoding/json"
	"fmt"
	"os"
	"pzrp/pkg/proto"
	"strconv"
	"strings"
)

type ClientConf struct {
	ServerAddr string `json:"server_addr"`
	ServerPort int    `json:"server_port"`
	Services   map[string]ClientServiceConf
}

type ClientServiceConf struct {
	Type       string `json:"type"`
	LocalIP    string `json:"local_ip"`
	LocalPort  string `json:"local_port"`
	RemotePort string `json:"remote_port"`
}

func (conf ClientServiceConf) GetProtocol() uint8 {
	protoName := strings.ToLower(conf.Type)
	p, ok := proto.StrToProto[protoName]
	if ok {
		return p
	}
	return proto.PROTO_NUL
}

type PortRange struct {
	Len       int
	rangeList [][]int
}

func NewPortRange(ports string) (*PortRange, error) {
	rst := &PortRange{
		Len:       0,
		rangeList: [][]int{},
	}
	for _, ss := range strings.Split(ports, ",") {
		port, err := strconv.Atoi(strings.Trim(ss, " "))
		if err == nil {
			rst.Len += 1
			rst.rangeList = append(rst.rangeList, []int{port})
		} else {
			syntaxError := fmt.Errorf("syntax error: %s", ss)
			pr := strings.Split(ss, "-")
			if len(pr) != 2 {
				return nil, syntaxError
			}
			minPort, err := strconv.Atoi(strings.Trim(pr[0], " "))
			if err != nil {
				return nil, syntaxError
			}
			maxPort, err := strconv.Atoi(strings.Trim(pr[1], " "))
			if err != nil {
				return nil, syntaxError
			}
			if minPort >= maxPort {
				return nil, syntaxError
			}
			rst.Len += (maxPort - minPort + 1)
			rst.rangeList = append(rst.rangeList, []int{minPort, maxPort})
		}
	}
	return rst, nil
}

func (pra PortRange) GetIter() func() int {
	var (
		pos1 = 0
		pos2 = 0
	)
	return func() int {
		if pos1 >= len(pra.rangeList) {
			return -1
		}
		rg := pra.rangeList[pos1]
		minPort := rg[0]
		maxPort := rg[len(rg)-1]
		port := minPort + pos2
		if port == maxPort {
			pos1 += 1
			pos2 = 0
		} else {
			pos2 += 1
		}
		return port
	}
}

func RegisterService(conf *ClientConf, register func(uint8, uint16, uint16)) {
	for _, srv := range conf.Services {
		rp, err := NewPortRange(srv.RemotePort)
		if err != nil {
			panic(err)
		}
		lp, err := NewPortRange(srv.LocalPort)
		if err != nil {
			panic(err)
		}
		if lp.Len != rp.Len {
			panic("端口个数不对应")
		}
		rit := rp.GetIter()
		lit := lp.GetIter()
		for {
			rport := rit()
			lport := lit()
			if rport < 0 {
				break
			}
			register(srv.GetProtocol(), uint16(rport), uint16(lport))
		}
	}
}

func LoadClientConfig() (*ClientConf, error) {
	var conf ClientConf
	jsonStr, err := os.ReadFile("c.json")
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal(jsonStr, &conf)
	if err != nil {
		return nil, err
	}
	return &conf, nil
}
