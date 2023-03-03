package main

import (
	"ginrtsp/conf"
	"ginrtsp/server"
	"ginrtsp/service"
)

func main() {
	// 从配置文件读取配置, 初始化各个模块
	conf.Init()

	// 装载路由
	r := server.NewRouter()

	go service.WsManager.Start()
	r.Run(":3000")
}

func getIP() (string,[]byte) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		util.Log().Error("%v", err)
		return "",nil
	}
	for _, value := range addrs {
		if ipnet, ok := value.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				util.Log().Debug(ipnet.IP.String())
				return ipnet.IP.String(), ipnet.IP
			}
		}
	}
	return "",nil
}

func boardcastIP(ip []byte)  {
	laddr := net.UDPAddr{
		IP:   net.IPv4(ip[0], ip[1], ip[2], ip[3]), //局域网广播地址
		Port: 8081,
	}

	raddr := net.UDPAddr{
		IP:   net.IPv4(255, 255, 255, 255),//写局域网下分配IP，0.0.0.0可以用来测试
		Port: 8081,
	}

	conn, err := net.DialUDP("udp", &laddr, &raddr)
	if err != nil {
		util.Log().Error(err.Error())
	}
	defer conn.Close()

	for {
		conn.Write(ip)
		//util.Log().Debug("send ip:%v", ip)
		time.Sleep(5 * time.Second)
	}
}
