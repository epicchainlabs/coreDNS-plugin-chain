package checkers

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/plugin/pkg/log"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type (
	ICMPChecker struct {
		logger       log.P
		isPrivileged bool
		timeout      time.Duration
	}

	ICMPCheckerParams struct {
		IsPrivileged bool
		Timeout      time.Duration
	}

	icmpParams struct {
		Network       string
		ListenAddress string
		ReplyType     icmp.Type
		Proto         int
		Msg           []byte
	}
)

func ParseICMPParams(c *caddy.Controller) (*ICMPCheckerParams, error) {
	prm := &ICMPCheckerParams{}

	for c.NextBlock() {
		switch c.Val() {
		case "privileged":
			args := c.RemainingArgs()
			if len(args) != 0 {
				return nil, fmt.Errorf("'privileged' param is used as a flag, so it isn't expected any value, but got '%v'", args)
			}
			prm.IsPrivileged = true
		case "timeout":
			args := c.RemainingArgs()
			if len(args) != 1 {
				return nil, fmt.Errorf("'timeout' param is expected to have one value, but got '%v'", args)
			}
			value := args[0]
			timeout, err := time.ParseDuration(value)
			if err != nil || timeout <= 0 {
				return nil, fmt.Errorf("invalid timeout '%s'", value)
			}
			prm.Timeout = timeout
		default:
			return nil, fmt.Errorf("unknow ICMP parameter: '%s'", c.Val())
		}
	}

	return prm, nil
}

// NewICMPChecker creates icmp checker.
func NewICMPChecker(logger log.P, prm *ICMPCheckerParams) (*ICMPChecker, error) {
	if prm.Timeout <= 0 {
		prm.Timeout = defaultHTTPTimeout
	}

	if prm.IsPrivileged {
		logger.Warningf("you run icmp checker in privileged mode, make sure Coredns is running behalf of root")
	}

	return &ICMPChecker{
		logger:       logger,
		isPrivileged: prm.IsPrivileged,
		timeout:      prm.Timeout,
	}, nil
}

func (c ICMPChecker) Check(endpoint string) bool {
	isV4 := isIPv4(endpoint)
	ip := net.ParseIP(endpoint)
	if ip == nil {
		return false
	}

	prm, err := c.getConnParams(isV4)
	if err != nil {
		log.Debugf("failed to get icmp params: %s", err.Error())
		return false
	}

	conn, err := icmp.ListenPacket(prm.Network, prm.ListenAddress)
	if err != nil {
		log.Debugf("listen icpm packet %s: %s", prm.Network, err.Error())
		return false
	}
	defer conn.Close()

	if err = c.writeMsg(conn, prm.Msg, ip); err != nil {
		log.Debugf("write icmp msg: %s", err.Error())
		return false
	}

	if err = c.readMsg(conn, prm); err != nil {
		log.Debugf("read icmp msg: %s", err.Error())
		return false
	}

	return true
}

func (c ICMPChecker) writeMsg(conn *icmp.PacketConn, msg []byte, ip net.IP) error {
	if _, err := conn.WriteTo(msg, c.formAddress(ip)); err != nil {
		return fmt.Errorf("write to '%s': %w", ip.String(), err)
	}
	return nil
}

func (c ICMPChecker) readMsg(conn *icmp.PacketConn, prm icmpParams) error {
	rb := make([]byte, 1500)

	if err := conn.SetReadDeadline(time.Now().Add(c.timeout)); err != nil {
		return fmt.Errorf("set read deadline: %w", err)
	}

	n, peer, err := conn.ReadFrom(rb)
	if err != nil {
		return fmt.Errorf("read from: %w", err)
	}

	rm, err := icmp.ParseMessage(prm.Proto, rb[:n])
	if err != nil {
		return fmt.Errorf("parse msg: %w", err)
	}

	switch rm.Type {
	case prm.ReplyType:
		return nil
	default:
		return fmt.Errorf("got %+v from %v; want echo reply", rm, peer)
	}
}

func (c ICMPChecker) formAddress(ip net.IP) net.Addr {
	var addr net.Addr
	if c.isPrivileged {
		addr = &net.IPAddr{IP: ip}
	} else {
		addr = &net.UDPAddr{IP: ip}
	}
	return addr
}

func (c ICMPChecker) getConnParams(isV4 bool) (icmpParams, error) {
	prm := icmpParams{}

	msg := &icmp.Message{
		Code: 0,
		Body: &icmp.Echo{
			ID: os.Getpid() & 0xffff, Seq: 1,
			Data: []byte(""),
		},
	}

	if isV4 {
		msg.Type = ipv4.ICMPTypeEcho

		prm.ListenAddress = "0.0.0.0"
		prm.ReplyType = ipv4.ICMPTypeEchoReply
		prm.Proto = 1 // https://godoc.org/golang.org/x/net/internal/iana ProtocolICMP

		if c.isPrivileged {
			prm.Network = "ip4:icmp"
		} else {
			prm.Network = "udp4"
		}
	} else {
		msg.Type = ipv6.ICMPTypeEchoRequest

		prm.ListenAddress = "::"
		prm.ReplyType = ipv6.ICMPTypeEchoReply
		prm.Proto = 58 // https://godoc.org/golang.org/x/net/internal/iana ProtocolIPv6ICMP

		if c.isPrivileged {
			prm.Network = "ip6:ipv6-icmp"
		} else {
			prm.Network = "udp6"
		}
	}

	msgRaw, err := msg.Marshal(nil)
	if err != nil {
		return icmpParams{}, fmt.Errorf("marshal icmp msg: %w", err)
	}

	prm.Msg = msgRaw

	return prm, nil
}

// isIPv4 check is string ip representation is ip v4 format
// we don't use 'net.IP.To4() != nil' because of its inaccuracy
// https://github.com/asaskevich/govalidator/pull/100
func isIPv4(str string) bool {
	for i := 0; i < len(str); i++ {
		switch str[i] {
		case '.':
			return true
		case ':':
			return false
		}
	}
	return false
}
