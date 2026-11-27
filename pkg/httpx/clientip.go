package httpx

import (
	"net"

	"github.com/gofiber/fiber/v3"
)

func ClientIP(c fiber.Ctx) string {
	ips := c.IPs() // parsed X-Forwarded-For, left to right
	for i := len(ips) - 1; i >= 0; i-- {
		ip := net.ParseIP(ips[i])
		if ip == nil {
			continue
		}
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
			continue // one of our hops, keep walking left
		}
		return ips[i]
	}
	return c.IP() // no public IP in chain (e.g. internal traffic)
}
