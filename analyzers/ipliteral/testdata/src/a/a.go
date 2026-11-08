package a

import "net"

// A bare number is left alone: it is as likely to be an object identifier or
// a mask as it is to be an address.
const mask = "255.255.255.255"

func ssh() string {
	return "root@203.0.113.9" // want "IP address 203.0.113.9 written into source"
}

func dial() string {
	return "203.0.113.9:22" // want "IP address 203.0.113.9 written into source"
}

func url() string {
	return "http://198.51.100.4:8080/health" // want "IP address 198.51.100.4 written into source"
}

func v6() string {
	return "[2001:db8::1]:443" // want "IP address 2001:db8::1 written into source"
}

// Placeholders are not locations.
func loopback() string {
	return "127.0.0.1:8080"
}

func unspecified() string {
	return "0.0.0.0:8080"
}

func any6() string {
	return "[::]:8080"
}

// A host name is what an address should have been in the first place.
func name() string {
	return "db.internal:5432"
}

// A network is not an address.
func cidr() string {
	_, _, _ = net.ParseCIDR("203.0.113.0/24")
	return "203.0.113.0/24"
}
