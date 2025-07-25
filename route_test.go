//go:build linux
// +build linux

package netlink

import (
	"net"
	"os"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/vishvananda/netlink/nl"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

func TestRouteAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	routes, err = RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not listed properly")
	}

	dstIP := net.IPv4(192, 168, 0, 42)
	routeToDstIP, err := RouteGet(dstIP)
	if err != nil {
		t.Fatal(err)
	}

	if len(routeToDstIP) == 0 {
		t.Fatal("Default route not present")
	}
	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}

	// add default route test
	// equiv: default dev lo
	_, defaultDst, _ := net.ParseCIDR("0.0.0.0/0")
	route = Route{Dst: defaultDst, LinkIndex: link.Attrs().Index}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Dev default route not listed properly")
	}
	if err := RouteDel(&routes[0]); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Dev default route not removed properly")
	}

	// equiv: blackhole default
	route = Route{Dst: defaultDst, Type: unix.RTN_BLACKHOLE, Family: FAMILY_V4}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%+v", routes)

	if len(routes) != 1 {
		t.Fatal("Blackhole default route not listed properly")
	}

	if err := RouteDel(&routes[0]); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Blackhole default route not removed properly")
	}

	// equiv: prohibit default
	route = Route{Dst: defaultDst, Type: unix.RTN_PROHIBIT}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Prohibit default route not listed properly")
	}

	if err := RouteDel(&routes[0]); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Prohibit default route not removed properly")
	}
}

func TestRoute6AddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// create dummy interface
	// IPv6 route added to loopback interface will be unreachable
	la := NewLinkAttrs()
	la.Name = "dummy_route6"
	la.TxQLen = 1500
	dummy := &Dummy{LinkAttrs: la}
	if err := LinkAdd(dummy); err != nil {
		t.Fatal(err)
	}

	// get dummy interface
	link, err := LinkByName("dummy_route6")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// remember number of routes before adding
	// typically one route (fe80::/64) will be created when dummy_route6 is created
	routes, err := RouteList(link, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	nroutes := len(routes)

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.ParseIP("2001:db8::0"),
		Mask: net.CIDRMask(64, 128),
	}
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes+1 {
		t.Fatal("Route not added properly")
	}

	dstIP := net.ParseIP("2001:db8::1")
	routeToDstIP, err := RouteGet(dstIP)
	if err != nil {
		t.Fatal(err)
	}

	// cleanup route
	if len(routeToDstIP) == 0 {
		t.Fatal("Route not present")
	}
	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes {
		t.Fatal("Route not removed properly")
	}

	// add a default link route
	_, defaultDst, _ := net.ParseCIDR("::/0")
	route = Route{LinkIndex: link.Attrs().Index, Dst: defaultDst}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes+1 {
		t.Fatal("Default route not added properly")
	}

	// add a default link route
	for _, route := range routes {
		if route.Dst.String() == defaultDst.String() {
			if err := RouteDel(&route); err != nil {
				t.Fatal(err)
			}
		}
	}
	routes, err = RouteList(link, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes {
		t.Fatal("Default route not removed properly")
	}

	// add blackhole default link route
	routes, err = RouteList(nil, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	nroutes = len(routes)

	route = Route{Type: unix.RTN_BLACKHOLE, Dst: defaultDst}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(nil, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes+1 {
		t.Fatal("Blackhole default route not added properly")
	}

	// add blackhole default link route
	for _, route := range routes {
		if ipNetEqual(route.Dst, defaultDst) {
			if err := RouteDel(&route); err != nil {
				t.Fatal(err)
			}
		}
	}
	routes, err = RouteList(nil, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes {
		t.Fatal("Blackhole default route not removed properly")
	}

	// add prohibit default link route
	routes, err = RouteList(nil, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	nroutes = len(routes)

	route = Route{Type: unix.RTN_BLACKHOLE, Dst: defaultDst}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(nil, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes+1 {
		t.Fatal("Prohibit default route not added properly")
	}

	// add prohibit default link route
	for _, route := range routes {
		if ipNetEqual(route.Dst, defaultDst) {
			if err := RouteDel(&route); err != nil {
				t.Fatal(err)
			}
		}
	}
	routes, err = RouteList(nil, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != nroutes {
		t.Fatal("Prohibit default route not removed properly")
	}

	// cleanup dummy interface created for the test
	if err := LinkDel(link); err != nil {
		t.Fatal(err)
	}
}

func TestRouteChange(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}

	if err := RouteChange(&route); err == nil {
		t.Fatal("Route added while it should fail")
	}

	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	ip = net.IPv4(127, 1, 1, 2)
	route = Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteChange(&route); err != nil {
		t.Fatal(err)
	}

	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 1 || !routes[0].Src.Equal(ip) {
		t.Fatal("Route not changed properly")
	}

	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}
}

func TestRouteReplace(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	ip = net.IPv4(127, 1, 1, 2)
	route = Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteReplace(&route); err != nil {
		t.Fatal(err)
	}

	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 1 || !routes[0].Src.Equal(ip) {
		t.Fatal("Route not replaced properly")
	}

	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}
}

func TestRouteAppend(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	ip = net.IPv4(127, 1, 1, 2)
	route = Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAppend(&route); err != nil {
		t.Fatal(err)
	}

	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 2 || !routes[1].Src.Equal(ip) {
		t.Fatal("Route not append properly")
	}

	if err := RouteDel(&routes[0]); err != nil {
		t.Fatal(err)
	}
	if err := RouteDel(&routes[1]); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}
}

func TestRouteAddIncomplete(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	route := Route{LinkIndex: link.Attrs().Index}
	if err := RouteAdd(&route); err == nil {
		t.Fatal("Adding incomplete route should fail")
	}
}

// expectRouteUpdate returns whether the expected updated is received within one minute.
func expectRouteUpdate(ch <-chan RouteUpdate, t, f uint16, dst net.IP) bool {
	for {
		timeout := time.After(time.Minute)
		select {
		case update := <-ch:
			if update.Type == t &&
				update.NlFlags == f &&
				update.Route.Dst != nil &&
				update.Route.Dst.IP.Equal(dst) {
				return true
			}
		case <-timeout:
			return false
		}
	}
}

func TestRouteSubscribe(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	ch := make(chan RouteUpdate)
	done := make(chan struct{})
	defer close(done)
	if err := RouteSubscribe(ch, done); err != nil {
		t.Fatal(err)
	}

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}

	if !expectRouteUpdate(ch, unix.RTM_NEWROUTE, unix.NLM_F_EXCL|unix.NLM_F_CREATE, dst.IP) {
		t.Fatal("Add update not received as expected")
	}
	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	if !expectRouteUpdate(ch, unix.RTM_DELROUTE, 0, dst.IP) {
		t.Fatal("Del update not received as expected")
	}
}

func TestRouteSubscribeWithOptions(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	ch := make(chan RouteUpdate)
	done := make(chan struct{})
	defer close(done)
	var lastError error
	defer func() {
		if lastError != nil {
			t.Fatalf("Fatal error received during subscription: %v", lastError)
		}
	}()
	if err := RouteSubscribeWithOptions(ch, done, RouteSubscribeOptions{
		ErrorCallback: func(err error) {
			lastError = err
		},
	}); err != nil {
		t.Fatal(err)
	}

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 1, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}

	if !expectRouteUpdate(ch, unix.RTM_NEWROUTE, unix.NLM_F_EXCL|unix.NLM_F_CREATE, dst.IP) {
		t.Fatal("Add update not received as expected")
	}
}

func TestRouteSubscribeAt(t *testing.T) {
	skipUnlessRoot(t)

	// Create an handle on a custom netns
	newNs, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	defer newNs.Close()

	nh, err := NewHandleAt(newNs)
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	// Subscribe for Route events on the custom netns
	ch := make(chan RouteUpdate)
	done := make(chan struct{})
	defer close(done)
	if err := RouteSubscribeAt(newNs, ch, done); err != nil {
		t.Fatal(err)
	}

	// get loopback interface
	link, err := nh.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = nh.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 169, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 100, 1, 1)
	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := nh.RouteAdd(&route); err != nil {
		t.Fatal(err)
	}

	if !expectRouteUpdate(ch, unix.RTM_NEWROUTE, unix.NLM_F_EXCL|unix.NLM_F_CREATE, dst.IP) {
		t.Fatal("Add update not received as expected")
	}
	if err := nh.RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	if !expectRouteUpdate(ch, unix.RTM_DELROUTE, 0, dst.IP) {
		t.Fatal("Del update not received as expected")
	}
}

func TestRouteSubscribeListExisting(t *testing.T) {
	skipUnlessRoot(t)

	// Create an handle on a custom netns
	newNs, err := netns.New()
	if err != nil {
		t.Fatal(err)
	}
	defer newNs.Close()

	nh, err := NewHandleAt(newNs)
	if err != nil {
		t.Fatal(err)
	}
	defer nh.Close()

	// get loopback interface
	link, err := nh.LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err = nh.LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route before subscribing
	dst10 := &net.IPNet{
		IP:   net.IPv4(10, 10, 10, 0),
		Mask: net.CIDRMask(24, 32),
	}

	ip := net.IPv4(127, 100, 1, 1)
	route10 := Route{LinkIndex: link.Attrs().Index, Dst: dst10, Src: ip}
	if err := nh.RouteAdd(&route10); err != nil {
		t.Fatal(err)
	}

	// Subscribe for Route events including existing routes
	ch := make(chan RouteUpdate)
	done := make(chan struct{})
	defer close(done)
	if err := RouteSubscribeWithOptions(ch, done, RouteSubscribeOptions{
		Namespace:    &newNs,
		ListExisting: true},
	); err != nil {
		t.Fatal(err)
	}

	if !expectRouteUpdate(ch, unix.RTM_NEWROUTE, 0, dst10.IP) {
		t.Fatal("Existing add update not received as expected")
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 169, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, Src: ip}
	if err := nh.RouteAdd(&route); err != nil {
		t.Fatal(err)
	}

	if !expectRouteUpdate(ch, unix.RTM_NEWROUTE, unix.NLM_F_EXCL|unix.NLM_F_CREATE, dst.IP) {
		t.Fatal("Add update not received as expected")
	}
	if err := nh.RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	if !expectRouteUpdate(ch, unix.RTM_DELROUTE, 0, dst.IP) {
		t.Fatal("Del update not received as expected")
	}
	if err := nh.RouteDel(&route10); err != nil {
		t.Fatal(err)
	}
	if !expectRouteUpdate(ch, unix.RTM_DELROUTE, 0, dst10.IP) {
		t.Fatal("Del update not received as expected")
	}
}

func TestRouteFilterAllTables(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(1, 1, 1, 1),
		Mask: net.CIDRMask(32, 32),
	}

	tables := []int{1000, 1001, 1002}
	src := net.IPv4(127, 3, 3, 3)
	for _, table := range tables {
		route := Route{
			LinkIndex: link.Attrs().Index,
			Dst:       dst,
			Src:       src,
			Scope:     unix.RT_SCOPE_LINK,
			Priority:  13,
			Table:     table,
			Type:      unix.RTN_UNICAST,
			Tos:       12,
			Hoplimit:  100,
			Realm:     328,
		}
		if err := RouteAdd(&route); err != nil {
			t.Fatal(err)
		}
	}
	routes, err := RouteListFiltered(FAMILY_V4, &Route{
		Dst:      dst,
		Src:      src,
		Scope:    unix.RT_SCOPE_LINK,
		Table:    unix.RT_TABLE_UNSPEC,
		Type:     unix.RTN_UNICAST,
		Tos:      12,
		Hoplimit: 100,
		Realm:    328,
	}, RT_FILTER_DST|RT_FILTER_SRC|RT_FILTER_SCOPE|RT_FILTER_TABLE|RT_FILTER_TYPE|RT_FILTER_TOS|RT_FILTER_HOPLIMIT|RT_FILTER_REALM)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 {
		t.Fatal("Routes not added properly")
	}

	for _, route := range routes {
		if route.Scope != unix.RT_SCOPE_LINK {
			t.Fatal("Invalid Scope. Route not added properly")
		}
		if route.Priority != 13 {
			t.Fatal("Invalid Priority. Route not added properly")
		}
		if !tableIDIn(tables, route.Table) {
			t.Fatalf("Invalid Table %d. Route not added properly", route.Table)
		}
		if route.Type != unix.RTN_UNICAST {
			t.Fatal("Invalid Type. Route not added properly")
		}
		if route.Tos != 12 {
			t.Fatal("Invalid Tos. Route not added properly")
		}
		if route.Hoplimit != 100 {
			t.Fatal("Invalid Hoplimit. Route not added properly")
		}
		if route.Realm != 328 {
			t.Fatal("Invalid Realm. Route not added properly")
		}
	}
}

func TestRouteFilterByFamily(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	const table int = 999

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a IPv4 gateway route
	dst4 := &net.IPNet{
		IP:   net.IPv4(2, 2, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}
	route4 := Route{LinkIndex: link.Attrs().Index, Dst: dst4, Table: table}
	if err := RouteAdd(&route4); err != nil {
		t.Fatal(err)
	}

	// add a IPv6 gateway route
	dst6 := &net.IPNet{
		IP:   net.ParseIP("2001:db9::0"),
		Mask: net.CIDRMask(64, 128),
	}
	route6 := Route{LinkIndex: link.Attrs().Index, Dst: dst6, Table: table}
	if err := RouteAdd(&route6); err != nil {
		t.Fatal(err)
	}

	// Get routes for both families
	routes_all, err := RouteListFiltered(FAMILY_ALL, &Route{Table: table}, RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes_all) != 2 {
		t.Fatal("Filtering by FAMILY_ALL doesn't find two routes")
	}

	// Get IPv4 route
	routes_v4, err := RouteListFiltered(FAMILY_V4, &Route{Table: table}, RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes_v4) != 1 {
		t.Fatal("Filtering by FAMILY_V4 doesn't find one route")
	}

	// Get IPv6 route
	routes_v6, err := RouteListFiltered(FAMILY_V6, &Route{Table: table}, RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes_v6) != 1 {
		t.Fatal("Filtering by FAMILY_V6 doesn't find one route")
	}

	// Get non-existent routes
	routes_non_existent, err := RouteListFiltered(99, &Route{Table: table}, RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes_non_existent) != 0 {
		t.Fatal("Filtering by non-existent family find some route")
	}
}

func TestRouteFilterIterCanStop(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(1, 1, 1, 1),
		Mask: net.CIDRMask(32, 32),
	}

	for i := 0; i < 3; i++ {
		route := Route{
			LinkIndex: link.Attrs().Index,
			Dst:       dst,
			Scope:     unix.RT_SCOPE_LINK,
			Priority:  1 + i,
			Table:     1000,
			Type:      unix.RTN_UNICAST,
		}
		if err := RouteAdd(&route); err != nil {
			t.Fatal(err)
		}
	}

	var routes []Route
	err = RouteListFilteredIter(FAMILY_V4, &Route{
		Dst:   dst,
		Scope: unix.RT_SCOPE_LINK,
		Table: 1000,
		Type:  unix.RTN_UNICAST,
	}, RT_FILTER_TABLE, func(route Route) (cont bool) {
		routes = append(routes, route)
		return len(routes) < 2
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatal("Unexpected number of iterations")
	}
	for _, route := range routes {
		if route.Scope != unix.RT_SCOPE_LINK {
			t.Fatal("Invalid Scope. Route not added properly")
		}
		if route.Priority < 1 || route.Priority > 3 {
			t.Fatal("Priority outside expected range. Route not added properly")
		}
		if route.Table != 1000 {
			t.Fatalf("Invalid Table %d. Route not added properly", route.Table)
		}
		if route.Type != unix.RTN_UNICAST {
			t.Fatal("Invalid Type. Route not added properly")
		}
	}
}

func BenchmarkRouteListFilteredNew(b *testing.B) {
	tearDown := setUpNetlinkTest(b)
	defer tearDown()

	link, err := setUpRoutesBench(b)

	b.ResetTimer()
	b.ReportAllocs()
	var routes []Route
	for i := 0; i < b.N; i++ {
		routes, err = pkgHandle.RouteListFiltered(FAMILY_V4, &Route{
			LinkIndex: link.Attrs().Index,
		}, RT_FILTER_OIF)
		if err != nil {
			b.Fatal(err)
		}
		if len(routes) != 65535 {
			b.Fatal("Incorrect number of routes.", len(routes))
		}
	}
	runtime.KeepAlive(routes)
}

func BenchmarkRouteListIter(b *testing.B) {
	tearDown := setUpNetlinkTest(b)
	defer tearDown()

	link, err := setUpRoutesBench(b)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var routes int
		err = RouteListFilteredIter(FAMILY_V4, &Route{
			LinkIndex: link.Attrs().Index,
		}, RT_FILTER_OIF, func(route Route) (cont bool) {
			routes++
			return true
		})
		if err != nil {
			b.Fatal(err)
		}
		if routes != 65535 {
			b.Fatal("Incorrect number of routes.", routes)
		}
	}
}

func setUpRoutesBench(b *testing.B) (Link, error) {
	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		b.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		b.Fatal(err)
	}

	// add a gateway route
	for i := 0; i < 65535; i++ {
		dst := &net.IPNet{
			IP:   net.IPv4(1, 1, byte(i>>8), byte(i&0xff)),
			Mask: net.CIDRMask(32, 32),
		}
		route := Route{
			LinkIndex: link.Attrs().Index,
			Dst:       dst,
			Scope:     unix.RT_SCOPE_LINK,
			Priority:  10,
			Type:      unix.RTN_UNICAST,
		}
		if err := RouteAdd(&route); err != nil {
			b.Fatal(err)
		}
	}
	return link, err
}

func tableIDIn(ids []int, id int) bool {
	for _, v := range ids {
		if v == id {
			return true
		}
	}
	return false
}

func TestRouteExtraFields(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(1, 1, 1, 1),
		Mask: net.CIDRMask(32, 32),
	}

	src := net.IPv4(127, 3, 3, 3)
	route := Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Src:       src,
		Scope:     unix.RT_SCOPE_LINK,
		Priority:  13,
		Table:     unix.RT_TABLE_MAIN,
		Type:      unix.RTN_UNICAST,
		Tos:       12,
		Hoplimit:  100,
		Realm:     239,
	}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteListFiltered(FAMILY_V4, &Route{
		Dst:      dst,
		Src:      src,
		Scope:    unix.RT_SCOPE_LINK,
		Table:    unix.RT_TABLE_MAIN,
		Type:     unix.RTN_UNICAST,
		Tos:      12,
		Hoplimit: 100,
		Realm:    239,
	}, RT_FILTER_DST|RT_FILTER_SRC|RT_FILTER_SCOPE|RT_FILTER_TABLE|RT_FILTER_TYPE|RT_FILTER_TOS|RT_FILTER_HOPLIMIT|RT_FILTER_REALM)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	if routes[0].Scope != unix.RT_SCOPE_LINK {
		t.Fatal("Invalid Scope. Route not added properly")
	}
	if routes[0].Priority != 13 {
		t.Fatal("Invalid Priority. Route not added properly")
	}
	if routes[0].Table != unix.RT_TABLE_MAIN {
		t.Fatal("Invalid Scope. Route not added properly")
	}
	if routes[0].Type != unix.RTN_UNICAST {
		t.Fatal("Invalid Type. Route not added properly")
	}
	if routes[0].Tos != 12 {
		t.Fatal("Invalid Tos. Route not added properly")
	}
	if routes[0].Hoplimit != 100 {
		t.Fatal("Invalid Hoplimit. Route not added properly")
	}
	if routes[0].Realm != 239 {
		t.Fatal("Invalid Realm. Route not added properly")
	}
}

func TestRouteMultiPath(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	idx := link.Attrs().Index
	route := Route{Dst: dst, MultiPath: []*NexthopInfo{{LinkIndex: idx}, {LinkIndex: idx}}}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("MultiPath Route not added properly")
	}
	if len(routes[0].MultiPath) != 2 {
		t.Fatal("MultiPath Route not added properly")
	}
}

func TestRouteIifOption(t *testing.T) {
	skipUnlessRoot(t)

	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	rootNs, err := netns.GetFromPid(1)
	if err != nil {
		t.Fatalf("could not get root ns: %s", err)
	}
	t.Cleanup(func() { rootNs.Close() })

	rootHdl, err := NewHandleAt(rootNs)
	if err != nil {
		t.Fatalf("could not create handle for root ns: %s", err)
	}
	t.Cleanup(func() { rootHdl.Close() })

	// setup a veth pair across two namespaces
	//   veth1 (2.2.2.3/24) <-> veth2 (2.2.2.4/24)

	// peer ns for veth pair
	ns, err := netns.New()
	if err != nil {
		t.Fatalf("could not create new ns: %s", err)
	}
	t.Cleanup(func() { ns.Close() })

	l := &Veth{
		LinkAttrs:     LinkAttrs{Name: "veth1"},
		PeerName:      "veth2",
		PeerNamespace: NsFd(ns),
	}
	if err = rootHdl.LinkAdd(l); err != nil {
		t.Fatalf("could not add veth interface: %s", err)
	}
	t.Cleanup(func() { rootHdl.LinkDel(l) })

	ve1, err := rootHdl.LinkByName("veth1")
	if err != nil {
		t.Fatalf("could not get link veth1: %s", err)
	}

	err = rootHdl.AddrAdd(ve1, &Addr{IPNet: &net.IPNet{IP: net.ParseIP("2.2.2.3"), Mask: net.CIDRMask(24, 32)}})
	if err != nil {
		t.Fatalf("could not set address for veth1: %s", err)
	}

	nh, err := NewHandleAt(ns)
	if err != nil {
		t.Fatalf("could not get handle for ns %+v: %s", ns, err)
	}
	t.Cleanup(func() { nh.Close() })

	ve2, err := nh.LinkByName("veth2")
	if err != nil {
		t.Fatalf("could not get link veth2: %s", err)
	}

	err = nh.AddrAdd(ve2, &Addr{IPNet: &net.IPNet{IP: net.ParseIP("2.2.2.4"), Mask: net.CIDRMask(24, 32)}})
	if err != nil {
		t.Fatalf("could set address for veth2: %s", err)
	}

	if err = rootHdl.LinkSetUp(ve1); err != nil {
		t.Fatalf("could not set veth1 up: %s", err)
	}

	if err = nh.LinkSetUp(ve2); err != nil {
		t.Fatalf("could not set veth2 up: %s", err)
	}

	err = nh.RouteAdd(&Route{
		Dst: &net.IPNet{
			IP:   net.IPv4zero,
			Mask: net.CIDRMask(0, 32),
		},
		Gw: net.ParseIP("2.2.2.3"),
	})
	if err != nil {
		t.Fatalf("could not add default route to ns: %s", err)
	}

	// setup finished, now do the actual test

	_, err = rootHdl.RouteGetWithOptions(net.ParseIP("8.8.8.8"), &RouteGetOptions{
		SrcAddr: net.ParseIP("2.2.2.4"),
	})
	if err == nil {
		t.Fatal("route get should have resulted in error but did not")
	}

	testWithOptions := func(opts *RouteGetOptions) {
		routes, err := rootHdl.RouteGetWithOptions(net.ParseIP("8.8.8.8"), opts)
		if err != nil {
			t.Fatalf("could not get route: %s", err)
		}
		if len(routes) != 1 {
			t.Fatalf("did not get exactly one route, routes: %+v", routes)
		}

		// should be the default route
		r, err := rootHdl.RouteGet(net.ParseIP("8.8.8.8"))
		if err != nil {
			t.Fatalf("could not get default route for 8.8.8.8: %s", err)
		}
		if len(r) != 1 {
			t.Fatalf("did not get exactly one route, routes: %+v", routes)
		}
		if !routes[0].Gw.Equal(r[0].Gw) {
			t.Fatalf("wrong gateway in route: expected: %s, got: %s", r[0].Gw, routes[0].Gw)
		}
		if routes[0].LinkIndex != r[0].LinkIndex {
			t.Fatalf("wrong link in route: expected: %d, got: %d", r[0].LinkIndex, routes[0].LinkIndex)
		}
	}

	t.Run("with iif", func(t *testing.T) {
		testWithOptions(&RouteGetOptions{
			SrcAddr: net.ParseIP("2.2.2.4"),
			Iif:     "veth1",
		})
	})

	t.Run("with iifIndex", func(t *testing.T) {
		testWithOptions(&RouteGetOptions{
			SrcAddr:  net.ParseIP("2.2.2.4"),
			IifIndex: ve1.Attrs().Index,
		})
	})

	t.Run("with iif and iifIndex", func(t *testing.T) {
		testWithOptions(&RouteGetOptions{
			SrcAddr:  net.ParseIP("2.2.2.4"),
			Iif:      "veth1",
			IifIndex: ve2.Attrs().Index, // Iif will supersede here
		})
	})
}

func TestRouteOifOption(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// setup two interfaces: eth0, eth1
	err := LinkAdd(&Dummy{LinkAttrs{Name: "eth0"}})
	if err != nil {
		t.Fatal(err)
	}

	link1, err := LinkByName("eth0")
	if err != nil {
		t.Fatal(err)
	}

	if err = LinkSetUp(link1); err != nil {
		t.Fatal(err)
	}

	if err = LinkAdd(&Dummy{LinkAttrs{Name: "eth1"}}); err != nil {
		t.Fatal(err)
	}

	link2, err := LinkByName("eth1")
	if err != nil {
		t.Fatal(err)
	}

	if err = LinkSetUp(link2); err != nil {
		t.Fatal(err)
	}

	// config ip addresses on interfaces
	addr1 := &Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(192, 168, 1, 1),
			Mask: net.CIDRMask(24, 32),
		},
	}

	if err = AddrAdd(link1, addr1); err != nil {
		t.Fatal(err)
	}

	addr2 := &Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(192, 168, 2, 1),
			Mask: net.CIDRMask(24, 32),
		},
	}

	if err = AddrAdd(link2, addr2); err != nil {
		t.Fatal(err)
	}

	// add default multipath route
	dst := &net.IPNet{
		IP:   net.IPv4(0, 0, 0, 0),
		Mask: net.CIDRMask(0, 32),
	}
	gw1 := net.IPv4(192, 168, 1, 254)
	gw2 := net.IPv4(192, 168, 2, 254)
	route := Route{Dst: dst, MultiPath: []*NexthopInfo{{LinkIndex: link1.Attrs().Index,
		Gw: gw1}, {LinkIndex: link2.Attrs().Index, Gw: gw2}}}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}

	// check getting route from specified Oif
	dstIP := net.IPv4(10, 1, 1, 1)
	routes, err := RouteGetWithOptions(dstIP, &RouteGetOptions{Oif: "eth0"})
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 1 || routes[0].LinkIndex != link1.Attrs().Index ||
		!routes[0].Gw.Equal(gw1) {
		t.Fatal("Get route from unmatched interface")
	}

	// check getting route from specified Oifindex
	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{OifIndex: link1.Attrs().Index})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].LinkIndex != link1.Attrs().Index ||
		!routes[0].Gw.Equal(gw1) {
		t.Fatal("Get route from unmatched interface")
	}

	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{Oif: "eth1"})
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 1 || routes[0].LinkIndex != link2.Attrs().Index ||
		!routes[0].Gw.Equal(gw2) {
		t.Fatal("Get route from unmatched interface")
	}

	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{OifIndex: link2.Attrs().Index})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes[0].LinkIndex != link2.Attrs().Index ||
		!routes[0].Gw.Equal(gw2) {
		t.Fatal("Get route from unmatched interface")
	}
}

func TestFilterDefaultRoute(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	// bring the interface up
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	address := &Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(127, 0, 0, 2),
			Mask: net.CIDRMask(24, 32),
		},
	}
	if err = AddrAdd(link, address); err != nil {
		t.Fatal(err)
	}

	// Add default route
	gw := net.IPv4(127, 0, 0, 2)

	defaultRoute := Route{
		Dst: nil,
		Gw:  gw,
	}

	if err := RouteAdd(&defaultRoute); err != nil {
		t.Fatal(err)
	}

	// add an extra route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	extraRoute := Route{
		Dst: dst,
		Gw:  gw,
	}

	if err := RouteAdd(&extraRoute); err != nil {
		t.Fatal(err)
	}
	var filterTests = []struct {
		filter   *Route
		mask     uint64
		expected net.IP
	}{
		{
			&Route{Dst: nil},
			RT_FILTER_DST,
			gw,
		},
		{
			&Route{Dst: dst},
			RT_FILTER_DST,
			gw,
		},
	}

	for _, f := range filterTests {
		routes, err := RouteListFiltered(FAMILY_V4, f.filter, f.mask)
		if err != nil {
			t.Fatal(err)
		}
		if len(routes) != 1 {
			t.Fatal("Route not filtered properly")
		}
		if !routes[0].Gw.Equal(gw) {
			t.Fatal("Unexpected Gateway")
		}
	}

}

func TestMPLSRouteAddDel(t *testing.T) {
	tearDown := setUpMPLSNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	mplsDst := 100
	route := Route{
		LinkIndex: link.Attrs().Index,
		MPLSDst:   &mplsDst,
		NewDst: &MPLSDestination{
			Labels: []int{200, 300},
		},
	}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_MPLS)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_MPLS)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}

}

func TestIP6tnlRouteAddDel(t *testing.T) {
	_, err := RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	_, dst, err := net.ParseCIDR("192.168.99.0/24")
	if err != nil {
		t.Fatalf("cannot parse destination prefix: %v", err)
	}

	encap := IP6tnlEncap{
		Dst: net.ParseIP("2001:db8::"),
		Src: net.ParseIP("::"),
	}

	route := &Route{
		LinkIndex: link.Attrs().Index,
		Dst:       dst,
		Encap:     &encap,
	}

	if err := RouteAdd(route); err != nil {
		t.Fatalf("Cannot add route: %v", err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	if err := RouteDel(route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}

}

func TestRouteEqual(t *testing.T) {
	mplsDst := 100
	seg6encap := &SEG6Encap{Mode: nl.SEG6_IPTUN_MODE_ENCAP}
	seg6encap.Segments = []net.IP{net.ParseIP("fc00:a000::11")}
	cases := []Route{
		{
			Dst: nil,
			Gw:  net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			ILinkIndex: 21,
			LinkIndex:  20,
			Dst:        nil,
			Gw:         net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Protocol:  20,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Priority:  20,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Type:      20,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Table:     200,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Tos:       1,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Hoplimit:  1,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Realm:     29,
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 20,
			Dst:       nil,
			Flags:     int(FLAG_ONLINK),
			Gw:        net.IPv4(1, 1, 1, 1),
		},
		{
			LinkIndex: 10,
			Dst: &net.IPNet{
				IP:   net.IPv4(192, 168, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			Src: net.IPv4(127, 1, 1, 1),
		},
		{
			LinkIndex: 10,
			Scope:     unix.RT_SCOPE_LINK,
			Dst: &net.IPNet{
				IP:   net.IPv4(192, 168, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			Src: net.IPv4(127, 1, 1, 1),
		},
		{
			LinkIndex: 3,
			Dst: &net.IPNet{
				IP:   net.IPv4(1, 1, 1, 1),
				Mask: net.CIDRMask(32, 32),
			},
			Src:      net.IPv4(127, 3, 3, 3),
			Scope:    unix.RT_SCOPE_LINK,
			Priority: 13,
			Table:    unix.RT_TABLE_MAIN,
			Type:     unix.RTN_UNICAST,
			Tos:      12,
		},
		{
			LinkIndex: 3,
			Dst: &net.IPNet{
				IP:   net.IPv4(1, 1, 1, 1),
				Mask: net.CIDRMask(32, 32),
			},
			Src:      net.IPv4(127, 3, 3, 3),
			Scope:    unix.RT_SCOPE_LINK,
			Priority: 13,
			Table:    unix.RT_TABLE_MAIN,
			Type:     unix.RTN_UNICAST,
			Hoplimit: 100,
		},
		{
			LinkIndex: 3,
			Dst: &net.IPNet{
				IP:   net.IPv4(1, 1, 1, 1),
				Mask: net.CIDRMask(32, 32),
			},
			Src:      net.IPv4(127, 3, 3, 3),
			Scope:    unix.RT_SCOPE_LINK,
			Priority: 13,
			Table:    unix.RT_TABLE_MAIN,
			Type:     unix.RTN_UNICAST,
			Realm:    129,
		},
		{
			LinkIndex: 10,
			MPLSDst:   &mplsDst,
			NewDst: &MPLSDestination{
				Labels: []int{200, 300},
			},
		},
		{
			Dst: nil,
			Gw:  net.IPv4(1, 1, 1, 1),
			Encap: &MPLSEncap{
				Labels: []int{100},
			},
		},
		{
			LinkIndex: 10,
			Dst: &net.IPNet{
				IP:   net.IPv4(10, 0, 0, 102),
				Mask: net.CIDRMask(32, 32),
			},
			Encap: seg6encap,
		},
		{
			Dst:       nil,
			MultiPath: []*NexthopInfo{{LinkIndex: 10}, {LinkIndex: 20}},
		},
		{
			Dst: nil,
			MultiPath: []*NexthopInfo{{
				LinkIndex: 10,
				Gw:        net.IPv4(1, 1, 1, 1),
			}, {LinkIndex: 20}},
		},
		{
			Dst: nil,
			MultiPath: []*NexthopInfo{{
				LinkIndex: 10,
				Gw:        net.IPv4(1, 1, 1, 1),
				Encap: &MPLSEncap{
					Labels: []int{100},
				},
			}, {LinkIndex: 20}},
		},
		{
			Dst: nil,
			MultiPath: []*NexthopInfo{{
				LinkIndex: 10,
				NewDst: &MPLSDestination{
					Labels: []int{200, 300},
				},
			}, {LinkIndex: 20}},
		},
		{
			Dst: nil,
			MultiPath: []*NexthopInfo{{
				LinkIndex: 10,
				Encap:     seg6encap,
			}, {LinkIndex: 20}},
		},
	}
	for i1 := range cases {
		for i2 := range cases {
			got := cases[i1].Equal(cases[i2])
			expected := i1 == i2
			if got != expected {
				t.Errorf("Equal(%q,%q) == %s but expected %s",
					cases[i1], cases[i2],
					strconv.FormatBool(got),
					strconv.FormatBool(expected))
			}
		}
	}
}

func TestIPNetEqual(t *testing.T) {
	cases := []string{
		"1.1.1.1/24", "1.1.1.0/24", "1.1.1.1/32",
		"0.0.0.0/0", "0.0.0.0/14",
		"2001:db8::/32", "2001:db8::/128",
		"2001:db8::caff/32", "2001:db8::caff/128",
		"",
	}
	for _, c1 := range cases {
		var n1 *net.IPNet
		if c1 != "" {
			var i1 net.IP
			var err1 error
			i1, n1, err1 = net.ParseCIDR(c1)
			if err1 != nil {
				panic(err1)
			}
			n1.IP = i1
		}
		for _, c2 := range cases {
			var n2 *net.IPNet
			if c2 != "" {
				var i2 net.IP
				var err2 error
				i2, n2, err2 = net.ParseCIDR(c2)
				if err2 != nil {
					panic(err2)
				}
				n2.IP = i2
			}

			got := ipNetEqual(n1, n2)
			expected := c1 == c2
			if got != expected {
				t.Errorf("IPNetEqual(%q,%q) == %s but expected %s",
					c1, c2,
					strconv.FormatBool(got),
					strconv.FormatBool(expected))
			}
		}
	}
}

func TestSEG6LocalEqual(t *testing.T) {
	// Different attributes exists in different Actions. For example, Action
	// SEG6_LOCAL_ACTION_END_X has In6Addr, SEG6_LOCAL_ACTION_END_T has Table etc.
	segs := []net.IP{net.ParseIP("fc00:a000::11")}
	// set flags for each actions.
	var flags_end [nl.SEG6_LOCAL_MAX]bool
	flags_end[nl.SEG6_LOCAL_ACTION] = true
	var flags_end_x [nl.SEG6_LOCAL_MAX]bool
	flags_end_x[nl.SEG6_LOCAL_ACTION] = true
	flags_end_x[nl.SEG6_LOCAL_NH6] = true
	var flags_end_t [nl.SEG6_LOCAL_MAX]bool
	flags_end_t[nl.SEG6_LOCAL_ACTION] = true
	flags_end_t[nl.SEG6_LOCAL_TABLE] = true
	var flags_end_dx2 [nl.SEG6_LOCAL_MAX]bool
	flags_end_dx2[nl.SEG6_LOCAL_ACTION] = true
	flags_end_dx2[nl.SEG6_LOCAL_OIF] = true
	var flags_end_dx6 [nl.SEG6_LOCAL_MAX]bool
	flags_end_dx6[nl.SEG6_LOCAL_ACTION] = true
	flags_end_dx6[nl.SEG6_LOCAL_NH6] = true
	var flags_end_dx4 [nl.SEG6_LOCAL_MAX]bool
	flags_end_dx4[nl.SEG6_LOCAL_ACTION] = true
	flags_end_dx4[nl.SEG6_LOCAL_NH4] = true
	var flags_end_dt6 [nl.SEG6_LOCAL_MAX]bool
	flags_end_dt6[nl.SEG6_LOCAL_ACTION] = true
	flags_end_dt6[nl.SEG6_LOCAL_TABLE] = true
	var flags_end_dt46 [nl.SEG6_LOCAL_MAX]bool
	flags_end_dt46[nl.SEG6_LOCAL_ACTION] = true
	flags_end_dt46[nl.SEG6_LOCAL_VRFTABLE] = true
	var flags_end_dt4 [nl.SEG6_LOCAL_MAX]bool
	flags_end_dt4[nl.SEG6_LOCAL_ACTION] = true
	flags_end_dt4[nl.SEG6_LOCAL_TABLE] = true
	var flags_end_b6 [nl.SEG6_LOCAL_MAX]bool
	flags_end_b6[nl.SEG6_LOCAL_ACTION] = true
	flags_end_b6[nl.SEG6_LOCAL_SRH] = true
	var flags_end_b6_encaps [nl.SEG6_LOCAL_MAX]bool
	flags_end_b6_encaps[nl.SEG6_LOCAL_ACTION] = true
	flags_end_b6_encaps[nl.SEG6_LOCAL_SRH] = true
	var flags_end_bpf [nl.SEG6_LOCAL_MAX]bool
	flags_end_bpf[nl.SEG6_LOCAL_ACTION] = true
	flags_end_bpf[nl.SEG6_LOCAL_BPF] = true

	cases := []SEG6LocalEncap{
		{
			Flags:  flags_end,
			Action: nl.SEG6_LOCAL_ACTION_END,
		},
		{
			Flags:   flags_end_x,
			Action:  nl.SEG6_LOCAL_ACTION_END_X,
			In6Addr: net.ParseIP("2001:db8::1"),
		},
		{
			Flags:  flags_end_t,
			Action: nl.SEG6_LOCAL_ACTION_END_T,
			Table:  10,
		},
		{
			Flags:  flags_end_dx2,
			Action: nl.SEG6_LOCAL_ACTION_END_DX2,
			Oif:    20,
		},
		{
			Flags:   flags_end_dx6,
			Action:  nl.SEG6_LOCAL_ACTION_END_DX6,
			In6Addr: net.ParseIP("2001:db8::1"),
		},
		{
			Flags:  flags_end_dx4,
			Action: nl.SEG6_LOCAL_ACTION_END_DX4,
			InAddr: net.IPv4(192, 168, 10, 10),
		},
		{
			Flags:  flags_end_dt6,
			Action: nl.SEG6_LOCAL_ACTION_END_DT6,
			Table:  30,
		},
		{
			Flags:  flags_end_dt4,
			Action: nl.SEG6_LOCAL_ACTION_END_DT4,
			Table:  40,
		},
		{
			Flags:    flags_end_dt46,
			Action:   nl.SEG6_LOCAL_ACTION_END_DT46,
			VrfTable: 50,
		},
		{
			Flags:    flags_end_b6,
			Action:   nl.SEG6_LOCAL_ACTION_END_B6,
			Segments: segs,
		},
		{
			Flags:    flags_end_b6_encaps,
			Action:   nl.SEG6_LOCAL_ACTION_END_B6_ENCAPS,
			Segments: segs,
		},
	}

	// SEG6_LOCAL_ACTION_END_BPF
	endBpf := SEG6LocalEncap{
		Flags:  flags_end_bpf,
		Action: nl.SEG6_LOCAL_ACTION_END_BPF,
	}
	_ = endBpf.SetProg(1, "firewall")
	cases = append(cases, endBpf)

	for i1 := range cases {
		for i2 := range cases {
			got := cases[i1].Equal(&cases[i2])
			expected := i1 == i2
			if got != expected {
				t.Errorf("Equal(%v,%v) == %s but expected %s",
					cases[i1], cases[i2],
					strconv.FormatBool(got),
					strconv.FormatBool(expected))
			}
		}
	}
}
func TestSEG6RouteAddDel(t *testing.T) {
	if os.Getenv("CI") == "true" {
		t.Skipf("Fails in CI with: route_test.go:*: Invalid Type. SEG6_IPTUN_MODE_INLINE routes not added properly")
	}
	// add/del routes with LWTUNNEL_SEG6 to/from loopback interface.
	// Test both seg6 modes: encap (IPv4) & inline (IPv6).
	tearDown := setUpSEG6NetlinkTest(t)
	defer tearDown()

	// get loopback interface and bring it up
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	dst1 := &net.IPNet{ // INLINE mode must be IPv6 route
		IP:   net.ParseIP("2001:db8::1"),
		Mask: net.CIDRMask(128, 128),
	}
	dst2 := &net.IPNet{
		IP:   net.IPv4(10, 0, 0, 102),
		Mask: net.CIDRMask(32, 32),
	}
	var s1, s2 []net.IP
	s1 = append(s1, net.ParseIP("::")) // inline requires "::"
	s1 = append(s1, net.ParseIP("fc00:a000::12"))
	s1 = append(s1, net.ParseIP("fc00:a000::11"))
	s2 = append(s2, net.ParseIP("fc00:a000::22"))
	s2 = append(s2, net.ParseIP("fc00:a000::21"))
	e1 := &SEG6Encap{Mode: nl.SEG6_IPTUN_MODE_INLINE}
	e2 := &SEG6Encap{Mode: nl.SEG6_IPTUN_MODE_ENCAP}
	e1.Segments = s1
	e2.Segments = s2
	route1 := Route{LinkIndex: link.Attrs().Index, Dst: dst1, Encap: e1}
	route2 := Route{LinkIndex: link.Attrs().Index, Dst: dst2, Encap: e2}

	// Add SEG6 routes
	if err := RouteAdd(&route1); err != nil {
		t.Fatal(err)
	}
	if err := RouteAdd(&route2); err != nil {
		t.Fatal(err)
	}
	// SEG6_IPTUN_MODE_INLINE
	routes, err := RouteList(link, FAMILY_V6)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("SEG6 routes not added properly")
	}
	for _, route := range routes {
		if route.Encap == nil || route.Encap.Type() != nl.LWTUNNEL_ENCAP_SEG6 {
			t.Fatal("Invalid Type. SEG6_IPTUN_MODE_INLINE routes not added properly")
		}
	}
	// SEG6_IPTUN_MODE_ENCAP
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("SEG6 routes not added properly")
	}
	for _, route := range routes {
		if route.Encap.Type() != nl.LWTUNNEL_ENCAP_SEG6 {
			t.Fatal("Invalid Type. SEG6_IPTUN_MODE_ENCAP routes not added properly")
		}
	}

	// Del (remove) SEG6 routes
	if err := RouteDel(&route1); err != nil {
		t.Fatal(err)
	}
	if err := RouteDel(&route2); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("SEG6 routes not removed properly")
	}
}

// add/del routes with LWTUNNEL_ENCAP_SEG6_LOCAL to/from dummy interface.
func TestSEG6LocalRoute6AddDel(t *testing.T) {
	minKernelRequired(t, 4, 14)
	tearDown := setUpSEG6NetlinkTest(t)
	defer tearDown()

	// create dummy interface
	// IPv6 route added to loopback interface will be unreachable
	la := NewLinkAttrs()
	la.Name = "dummy_route6"
	la.TxQLen = 1500
	dummy := &Dummy{LinkAttrs: la}
	if err := LinkAdd(dummy); err != nil {
		t.Fatal(err)
	}
	// get dummy interface and bring it up
	link, err := LinkByName("dummy_route6")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	dst1 := &net.IPNet{
		IP:   net.ParseIP("2001:db8::1"),
		Mask: net.CIDRMask(128, 128),
	}

	// Create Route including Action SEG6_LOCAL_ACTION_END_B6.
	// Could be any Action but thought better to have seg list.
	var s1 []net.IP
	s1 = append(s1, net.ParseIP("fc00:a000::12"))
	s1 = append(s1, net.ParseIP("fc00:a000::11"))
	var flags_end_b6_encaps [nl.SEG6_LOCAL_MAX]bool
	flags_end_b6_encaps[nl.SEG6_LOCAL_ACTION] = true
	flags_end_b6_encaps[nl.SEG6_LOCAL_SRH] = true
	e1 := &SEG6LocalEncap{
		Flags:    flags_end_b6_encaps,
		Action:   nl.SEG6_LOCAL_ACTION_END_B6,
		Segments: s1,
	}
	route1 := Route{LinkIndex: link.Attrs().Index, Dst: dst1, Encap: e1}

	// Add SEG6Local routes
	if err := RouteAdd(&route1); err != nil {
		t.Fatal(err)
	}

	// typically one route (fe80::/64) will be created when dummy_route6 is created.
	// Thus you cannot use RouteList() to find the route entry just added.
	// Lookup route and confirm it's SEG6Local route just added.
	routesFound, err := RouteGet(dst1.IP)
	if err != nil {
		t.Fatal(err)
	}
	if len(routesFound) != 1 { // should only find 1 route entry
		t.Fatal("SEG6Local route not added correctly")
	}
	if !e1.Equal(routesFound[0].Encap) {
		t.Fatal("Encap does not match the original SEG6LocalEncap")
	}

	// Del SEG6Local routes
	if err := RouteDel(&route1); err != nil {
		t.Fatal(err)
	}
	// Confirm route is deleted.
	if _, err = RouteGet(dst1.IP); err == nil {
		t.Fatal("SEG6Local route still exists.")
	}

	// cleanup dummy interface created for the test
	if err := LinkDel(link); err != nil {
		t.Fatal(err)
	}
}

func TestBpfEncap(t *testing.T) {
	tCase := &BpfEncap{}
	if err := tCase.SetProg(nl.LWT_BPF_IN, 0, "test_in"); err == nil {
		t.Fatal("BpfEncap: inserting invalid FD did not return error")
	}
	if err := tCase.SetProg(nl.LWT_BPF_XMIT_HEADROOM, 23, "test_nout"); err == nil {
		t.Fatal("BpfEncap: inserting invalid mode did not return error")
	}
	if err := tCase.SetProg(nl.LWT_BPF_XMIT, 12, "test_xmit"); err != nil {
		t.Fatal("BpfEncap: inserting valid program option returned error")
	}
	if err := tCase.SetXmitHeadroom(12); err != nil {
		t.Fatal("BpfEncap: inserting valid headroom returned error")
	}
	if err := tCase.SetXmitHeadroom(nl.LWT_BPF_MAX_HEADROOM + 1); err == nil {
		t.Fatal("BpfEncap: inserting invalid headroom did not return error")
	}
	tCase = &BpfEncap{}

	expected := &BpfEncap{
		progs: [nl.LWT_BPF_MAX]bpfObj{
			1: {
				progName: "test_in[fd:10]",
				progFd:   10,
			},
			2: {
				progName: "test_out[fd:11]",
				progFd:   11,
			},
			3: {
				progName: "test_xmit[fd:21]",
				progFd:   21,
			},
		},
		headroom: 128,
	}

	_ = tCase.SetProg(1, 10, "test_in")
	_ = tCase.SetProg(2, 11, "test_out")
	_ = tCase.SetProg(3, 21, "test_xmit")
	_ = tCase.SetXmitHeadroom(128)
	if !tCase.Equal(expected) {
		t.Fatal("BpfEncap: equal comparison failed")
	}
	_ = tCase.SetProg(3, 21, "test2_xmit")
	if tCase.Equal(expected) {
		t.Fatal("BpfEncap: equal comparison succeeded when attributes differ")
	}
}

func TestMTURouteAddDel(t *testing.T) {
	_, err := RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, MTU: 500}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	if route.MTU != routes[0].MTU {
		t.Fatal("Route mtu not set properly")
	}

	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}
}

func TestMTULockRouteAddDel(t *testing.T) {
	_, err := RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, MTU: 500, MTULock: true}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	if route.MTU != routes[0].MTU {
		t.Fatal("Route MTU not set properly")
	}

	if route.MTULock != routes[0].MTULock {
		t.Fatal("Route MTU lock not set properly")
	}

	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}
}

func TestRtoMinLockRouteAddDel(t *testing.T) {
	_, err := RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// get loopback interface
	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	// bring the interface up
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	// add a gateway route
	dst := &net.IPNet{
		IP:   net.IPv4(192, 168, 0, 0),
		Mask: net.CIDRMask(24, 32),
	}

	route := Route{LinkIndex: link.Attrs().Index, Dst: dst, RtoMin: 40, RtoMinLock: true}
	if err := RouteAdd(&route); err != nil {
		t.Fatal(err)
	}
	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	if route.RtoMin != routes[0].RtoMin {
		t.Fatal("Route RtoMin not set properly")
	}

	if route.RtoMinLock != routes[0].RtoMinLock {
		t.Fatal("Route RtoMin lock not set properly")
	}

	if err := RouteDel(&route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}
}

func TestRouteViaAddDel(t *testing.T) {
	minKernelRequired(t, 5, 4)
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	_, err := RouteList(nil, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}

	link, err := LinkByName("lo")
	if err != nil {
		t.Fatal(err)
	}

	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	route := &Route{
		LinkIndex: link.Attrs().Index,
		Dst: &net.IPNet{
			IP:   net.IPv4(192, 168, 0, 0),
			Mask: net.CIDRMask(24, 32),
		},
		MultiPath: []*NexthopInfo{
			{
				LinkIndex: link.Attrs().Index,
				Via: &Via{
					AddrFamily: FAMILY_V6,
					Addr:       net.ParseIP("2001::1"),
				},
			},
		},
	}

	if err := RouteAdd(route); err != nil {
		t.Fatalf("route: %v, err: %v", route, err)
	}

	routes, err := RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatal("Route not added properly")
	}

	got := routes[0].Via
	want := route.MultiPath[0].Via
	if !want.Equal(got) {
		t.Fatalf("Route Via attribute does not match; got: %s, want: %s", got, want)
	}

	if err := RouteDel(route); err != nil {
		t.Fatal(err)
	}
	routes, err = RouteList(link, FAMILY_V4)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 0 {
		t.Fatal("Route not removed properly")
	}
}

func TestRouteUIDOption(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// setup eth0 so that network is reachable
	err := LinkAdd(&Dummy{LinkAttrs{Name: "eth0"}})
	if err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("eth0")
	if err != nil {
		t.Fatal(err)
	}
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	addr := &Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(192, 168, 1, 1),
			Mask: net.CIDRMask(16, 32),
		},
	}
	if err = AddrAdd(link, addr); err != nil {
		t.Fatal(err)
	}

	// a table different than unix.RT_TABLE_MAIN
	testtable := 1000

	gw1 := net.IPv4(192, 168, 1, 254)
	gw2 := net.IPv4(192, 168, 2, 254)

	// add default route via gw1 (in main route table by default)
	defaultRouteMain := Route{
		Dst: nil,
		Gw:  gw1,
	}
	if err := RouteAdd(&defaultRouteMain); err != nil {
		t.Fatal(err)
	}

	// add default route via gw2 in test route table
	defaultRouteTest := Route{
		Dst:   nil,
		Gw:    gw2,
		Table: testtable,
	}
	if err := RouteAdd(&defaultRouteTest); err != nil {
		t.Fatal(err)
	}

	// check the routes are in different tables
	routes, err := RouteListFiltered(FAMILY_V4, &Route{
		Dst:   nil,
		Table: unix.RT_TABLE_UNSPEC,
	}, RT_FILTER_DST|RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].Table == routes[1].Table {
		t.Fatal("Routes not added properly")
	}

	// add a rule that uidrange match should result in route lookup of test table for uid other than current
	// current uid is 0 due to skipUnlessRoot()
	var uid uint32 = 1000
	rule := NewRule()
	rule.UIDRange = NewRuleUIDRange(uid, uid)
	rule.Table = testtable
	if err := RuleAdd(rule); err != nil {
		t.Fatal(err)
	}

	dstIP := net.IPv4(10, 1, 1, 1)

	// check getting route without UID option
	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{UID: nil})
	if err != nil {
		t.Fatal(err)
	}
	// current uid is outside uidrange; rule does not apply; lookup main table
	if len(routes) != 1 || !routes[0].Gw.Equal(gw1) {
		t.Fatal(routes)
	}

	// check getting route with UID option
	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{UID: &uid})
	if err != nil {
		t.Fatal(err)
	}
	// option uid is within uidrange; rule applies; lookup test table
	if len(routes) != 1 || !routes[0].Gw.Equal(gw2) {
		t.Fatal(routes)
	}
}

func TestRouteFWMarkOption(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	// setup eth0 so that network is reachable
	err := LinkAdd(&Dummy{LinkAttrs{Name: "eth0"}})
	if err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("eth0")
	if err != nil {
		t.Fatal(err)
	}
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	addr := &Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(192, 168, 1, 1),
			Mask: net.CIDRMask(16, 32),
		},
	}
	if err = AddrAdd(link, addr); err != nil {
		t.Fatal(err)
	}

	// a table different than unix.RT_TABLE_MAIN
	testTable0 := 254
	testTable1 := 1000
	testTable2 := 1001

	gw0 := net.IPv4(192, 168, 1, 254)
	gw1 := net.IPv4(192, 168, 2, 254)
	gw2 := net.IPv4(192, 168, 3, 254)

	// add default route via gw0 (in main route table by default)
	defaultRouteMain := Route{
		Dst:   nil,
		Gw:    gw0,
		Table: testTable0,
	}
	if err := RouteAdd(&defaultRouteMain); err != nil {
		t.Fatal(err)
	}

	// add default route via gw1 in test route table
	defaultRouteTest1 := Route{
		Dst:   nil,
		Gw:    gw1,
		Table: testTable1,
	}
	if err := RouteAdd(&defaultRouteTest1); err != nil {
		t.Fatal(err)
	}

	// add default route via gw2 in test route table
	defaultRouteTest2 := Route{
		Dst:   nil,
		Gw:    gw2,
		Table: testTable2,
	}
	if err := RouteAdd(&defaultRouteTest2); err != nil {
		t.Fatal(err)
	}

	// check the routes are in different tables
	routes, err := RouteListFiltered(FAMILY_V4, &Route{
		Dst:   nil,
		Table: unix.RT_TABLE_UNSPEC,
	}, RT_FILTER_DST|RT_FILTER_TABLE)
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 || routes[0].Table == routes[1].Table || routes[1].Table == routes[2].Table ||
		routes[0].Table == routes[2].Table {
		t.Fatal("Routes not added properly")
	}

	// add a rule that fwmark match should result in route lookup of test table
	fwmark1 := uint32(0xAFFFFFFF)
	fwmark2 := uint32(0xBFFFFFFF)

	rule := NewRule()
	rule.Mark = fwmark1
	rule.Mask = &[]uint32{0xFFFFFFFF}[0]

	rule.Table = testTable1
	if err := RuleAdd(rule); err != nil {
		t.Fatal(err)
	}

	rule = NewRule()
	rule.Mark = fwmark2
	rule.Mask = &[]uint32{0xFFFFFFFF}[0]
	rule.Table = testTable2
	if err := RuleAdd(rule); err != nil {
		t.Fatal(err)
	}

	rules, err := RuleListFiltered(FAMILY_V4, &Rule{Mark: fwmark1}, RT_FILTER_MARK)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Table != testTable1 || rules[0].Mark != fwmark1 {
		t.Fatal("Rules not added properly")
	}

	rules, err = RuleListFiltered(FAMILY_V4, &Rule{Mark: fwmark2}, RT_FILTER_MARK)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Table != testTable2 || rules[0].Mark != fwmark2 {
		t.Fatal("Rules not added properly")
	}

	dstIP := net.IPv4(10, 1, 1, 1)

	// check getting route without FWMark option
	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || !routes[0].Gw.Equal(gw0) {
		t.Fatal(routes)
	}

	// check getting route with FWMark option
	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{Mark: fwmark1})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || !routes[0].Gw.Equal(gw1) {
		t.Fatal(routes)
	}

	// check getting route with FWMark option
	routes, err = RouteGetWithOptions(dstIP, &RouteGetOptions{Mark: fwmark2})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || !routes[0].Gw.Equal(gw2) {
		t.Fatal(routes)
	}
}

func TestRouteGetFIBMatchOption(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	err := LinkAdd(&Dummy{LinkAttrs{Name: "eth0"}})
	if err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("eth0")
	if err != nil {
		t.Fatal(err)
	}
	if err = LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	addr := &Addr{
		IPNet: &net.IPNet{
			IP:   net.IPv4(192, 168, 0, 2),
			Mask: net.CIDRMask(24, 32),
		},
	}
	if err = AddrAdd(link, addr); err != nil {
		t.Fatal(err)
	}

	route := &Route{
		LinkIndex: link.Attrs().Index,
		Gw:        net.IPv4(192, 168, 1, 1),
		Dst: &net.IPNet{
			IP:   net.IPv4(192, 168, 2, 0),
			Mask: net.CIDRMask(24, 32),
		},
		Flags: int(FLAG_ONLINK),
	}

	err = RouteAdd(route)
	if err != nil {
		t.Fatal(err)
	}

	routes, err := RouteGetWithOptions(net.IPv4(192, 168, 2, 1), &RouteGetOptions{FIBMatch: true})
	if err != nil {
		t.Fatal(err)
	}

	if len(routes) != 1 {
		t.Fatalf("More than one route matched %v", routes)
	}

	if len(routes[0].ListFlags()) != 1 {
		t.Fatalf("More than one route flag returned %v", routes[0].ListFlags())
	}

	flag := routes[0].ListFlags()[0]
	if flag != "onlink" {
		t.Fatalf("Unexpected flag %s returned", flag)
	}
}
