package rules

import (
	"net/netip"

	"github.com/oschwald/geoip2-golang"

	C "github.com/fossabot/clash/constant"

	log "github.com/sirupsen/logrus"
)

var mmdb *geoip2.Reader

func init() {
	var err error
	mmdb, err = geoip2.Open(C.MMDBPath)
	if err != nil {
		log.Fatalf("Can't load mmdb: %s", err.Error())
	}
}

type GEOIP struct {
	country string
	adapter string
}

func (g *GEOIP) RuleType() C.RuleType {
	return C.GEOIP
}

func (g *GEOIP) IsMatch(addr *C.Addr) bool {
	if addr.IP == nil {
		return false
	}

	ip, ok := netip.AddrFromSlice(*addr.IP)
	if !ok {
		return false
	}

	record, _ := mmdb.Country(ip)
	return record.Country.ISOCode == g.country
}

func (g *GEOIP) Adapter() string {
	return g.adapter
}

func (g *GEOIP) Payload() string {
	return g.country
}

func NewGEOIP(country string, adapter string) *GEOIP {
	return &GEOIP{
		country: country,
		adapter: adapter,
	}
}
