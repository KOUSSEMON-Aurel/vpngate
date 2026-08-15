package tui

// capNetAdmin is the Linux capability required to create a tun interface.
// openvpn needs it (in addition to an accessible /dev/net/tun) to run the
// tunnel, which is why connect must run as root or with setcap.
const capNetAdmin = uint64(1) << 12

// privState describes whether the current process can create a tun
// interface: as root (sudo), via CAP_NET_ADMIN on the vpngate or openvpn
// binary, or not at all.
type privState int

const (
	// privUnknown is used on platforms where privilege detection is not
	// implemented; it renders no badge.
	privUnknown privState = iota
	// privRoot means the process runs with euid 0 (sudo or root shell).
	privRoot
	// privCapNetAdmin means the vpngate or openvpn binary carries the
	// CAP_NET_ADMIN file capability, so no sudo is needed.
	privCapNetAdmin
	// privNone means the process cannot create a tun interface: openvpn
	// will fail at connection time with TUNSETIFF "Operation not
	// permitted".
	privNone
)
