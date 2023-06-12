package eureka_client

import (
	"net"
)

// GetLocalIP 获取本地 ip
func GetLocalIP() (ip string) {
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		return
	}
	for _, address := range addresses {
		// check the address type and if it is not a loopback the display it
		if ipNet, ok := address.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && !ipNet.IP.IsLinkLocalUnicast() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String()
			}
		}
	}
	return
}
