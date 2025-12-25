# Environment Variables

#### A comprehensive list of every environment variable that has any effect on WireGuard Pro or BoringTun

WG_PORT="51820" # MANDATORY 
WG_ENDPOINT="foo.example.com" # MANDATORY

WG_ALLOWED_IPS="0.0.0.0/0, ::/0" # All IPs allowed as default
WG_DNS_SERVER="1.1.1.1"
WG_IPV4_BASE_ADDR="10.8.0.1" # Base IP address for IPv4 (wg0 interface IP), needs to be the first address in subnet
WG_IPV6_BASE_ADDR="fd86:ea04:1111::1" # Base IP address for IPv6 (wg0 interface IP), needs to be the first address in subnet
ddWG_PORT="51820" # MANDATORY 
