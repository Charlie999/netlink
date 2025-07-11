//go:build linux
// +build linux

package netlink

import (
	"net"
	"reflect"
	"testing"
	"time"

	"github.com/charlie999/netlink/nl"
	"golang.org/x/sys/unix"
)

func TestFilterAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Ingress)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}
	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_IP,
		},
		RedirIndex: redir.Attrs().Index,
		ClassId:    classId,
	}
	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}
	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}
	if u32.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}
	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}
	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterReplace(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}

	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_IP,
		},
		RedirIndex: redir.Attrs().Index,
		ClassId:    MakeHandle(1, 1),
	}

	if err := FilterReplace(filter); err != nil {
		t.Fatal(err)
	}
	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed replace filter")
	}

	if err := FilterReplace(filter); err != nil {
		t.Fatal(err)
	}
}

func TestAdvancedFilterAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "baz"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("baz")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	index := link.Attrs().Index

	qdiscHandle := MakeHandle(0x1, 0x0)
	qdiscAttrs := QdiscAttrs{
		LinkIndex: index,
		Handle:    qdiscHandle,
		Parent:    HANDLE_ROOT,
	}

	qdisc := NewHtb(qdiscAttrs)
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Htb)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}

	classId := MakeHandle(0x1, 0x46cb)
	classAttrs := ClassAttrs{
		LinkIndex: index,
		Parent:    qdiscHandle,
		Handle:    classId,
	}
	htbClassAttrs := HtbClassAttrs{
		Rate:   512 * 1024,
		Buffer: 32 * 1024,
	}
	htbClass := NewHtbClass(classAttrs, htbClassAttrs)
	if err = ClassReplace(htbClass); err != nil {
		t.Fatalf("Failed to add a HTB class: %v", err)
	}
	classes, err := SafeClassList(link, qdiscHandle)
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 1 {
		t.Fatal("Failed to add class")
	}
	_, ok = classes[0].(*HtbClass)
	if !ok {
		t.Fatal("Class is the wrong type")
	}

	htid := MakeHandle(0x0010, 0000)
	divisor := uint32(1)
	hashTable := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: index,
			Handle:    htid,
			Parent:    qdiscHandle,
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		Divisor: divisor,
	}
	cHashTable := *hashTable
	if err := FilterAdd(hashTable); err != nil {
		t.Fatal(err)
	}
	// Check if the hash table is identical before and after FilterAdd.
	if !reflect.DeepEqual(cHashTable, *hashTable) {
		t.Fatalf("Hash table %v and %v are not equal", cHashTable, *hashTable)
	}

	u32SelKeys := []TcU32Key{
		{
			Mask:    0xff,
			Val:     80,
			Off:     20,
			OffMask: 0,
		},
		{
			Mask:    0xffff,
			Val:     0x146ca,
			Off:     32,
			OffMask: 0,
		},
	}

	handle := MakeHandle(0x0000, 0001)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: index,
			Handle:    handle,
			Parent:    qdiscHandle,
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		Sel: &TcU32Sel{
			Keys:  u32SelKeys,
			Flags: TC_U32_TERMINAL,
		},
		ClassId: classId,
		Hash:    htid,
		Actions: []Action{},
	}
	// Copy filter.
	cFilter := *filter
	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}
	// Check if the filter is identical before and after FilterAdd.
	if !reflect.DeepEqual(cFilter, *filter) {
		t.Fatalf("U32 %v and %v are not equal", cFilter, *filter)
	}

	filters, err := FilterList(link, qdiscHandle)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}

	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}
	// Endianness checks
	if u32.Sel.Offmask != filter.Sel.Offmask {
		t.Fatal("The endianness of TcU32Key.Sel.Offmask is wrong")
	}
	if u32.Sel.Hmask != filter.Sel.Hmask {
		t.Fatal("The endianness of TcU32Key.Sel.Hmask is wrong")
	}
	for i, key := range u32.Sel.Keys {
		if key.Mask != filter.Sel.Keys[i].Mask {
			t.Fatal("The endianness of TcU32Key.Mask is wrong")
		}
		if key.Val != filter.Sel.Keys[i].Val {
			t.Fatal("The endianness of TcU32Key.Val is wrong")
		}
	}
	if u32.Handle != (handle | htid) {
		t.Fatalf("The handle is wrong. expected %v but actually %v",
			(handle | htid), u32.Handle)
	}
	if u32.Hash != htid {
		t.Fatal("The hash table ID is wrong")
	}

	if err := FilterDel(u32); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, qdiscHandle)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err = ClassDel(htbClass); err != nil {
		t.Fatalf("Failed to delete a HTP class: %v", err)
	}
	classes, err = SafeClassList(link, qdiscHandle)
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 0 {
		t.Fatal("Failed to remove class")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterFwAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	attrs := QdiscAttrs{
		LinkIndex: link.Attrs().Index,
		Handle:    MakeHandle(0xffff, 0),
		Parent:    HANDLE_ROOT,
	}
	qdisc := NewHtb(attrs)
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Htb)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}

	classattrs := ClassAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    MakeHandle(0xffff, 0),
		Handle:    MakeHandle(0xffff, 2),
	}

	htbclassattrs := HtbClassAttrs{
		Rate:    1234000,
		Cbuffer: 1690,
	}
	class := NewHtbClass(classattrs, htbclassattrs)
	if err := ClassAdd(class); err != nil {
		t.Fatal(err)
	}
	classes, err := SafeClassList(link, MakeHandle(0xffff, 2))
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 1 {
		t.Fatal("Failed to add class")
	}

	police := NewPoliceAction()
	police.Burst = 12345
	police.Rate = 1234
	police.PeakRate = 2345
	police.Action = TcAct(TC_POLICE_SHOT)

	filterattrs := FilterAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    MakeHandle(0xffff, 0),
		Handle:    MakeHandle(0, 0x6),
		Priority:  1,
		Protocol:  unix.ETH_P_IP,
	}

	filter := FwFilter{
		FilterAttrs: filterattrs,
		ClassId:     MakeHandle(0xffff, 2),
		Police:      police,
	}

	if err := FilterAdd(&filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	fw, ok := filters[0].(*FwFilter)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}
	if fw.Police.Rate != filter.Police.Rate {
		t.Fatal("Police Rate doesn't match")
	}
	if fw.ClassId != filter.ClassId {
		t.Fatal("ClassId doesn't match")
	}
	if fw.InDev != filter.InDev {
		t.Fatal("InDev doesn't match")
	}
	if fw.Police.AvRate != filter.Police.AvRate {
		t.Fatal("AvRate doesn't match")
	}

	if err := FilterDel(&filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}
	if err := ClassDel(class); err != nil {
		t.Fatal(err)
	}
	classes, err = SafeClassList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(classes) != 0 {
		t.Fatal("Failed to remove class")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterFwActAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Ingress)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}

	classId := MakeHandle(1, 1)
	filter := &FwFilter{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
			Handle:    MakeHandle(0, 0x6),
		},
		ClassId: classId,
		Actions: []Action{
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	fw, ok := filters[0].(*FwFilter)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if len(fw.Actions) != 1 {
		t.Fatalf("Too few Actions in filter")
	}
	if fw.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}

	mia, ok := fw.Actions[0].(*MirredAction)
	if !ok {
		t.Fatal("Unable to find mirred action")
	}

	if mia.Attrs().Action != TC_ACT_STOLEN {
		t.Fatal("Mirred action isn't TC_ACT_STOLEN")
	}

	if mia.MirredAction != TCA_EGRESS_REDIR {
		t.Fatal("MirredAction isn't TCA_EGRESS_REDIR")
	}

	if mia.Ifindex != redir.Attrs().Index {
		t.Fatal("Unmatched redirect index")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterU32BpfAddDel(t *testing.T) {
	t.Skipf("Fd does not match in ci")
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Ingress)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}

	fd, err := loadSimpleBpf(BPF_PROG_TYPE_SCHED_ACT, 1)
	if err != nil {
		t.Skipf("Loading bpf program failed: %s", err)
	}
	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Actions: []Action{
			&BpfAction{Fd: fd, Name: "simple"},
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if len(u32.Actions) != 2 {
		t.Fatalf("Too few Actions in filter")
	}
	if u32.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}

	// actions can be returned in reverse order
	bpfAction, ok := u32.Actions[0].(*BpfAction)
	if !ok {
		bpfAction, ok = u32.Actions[1].(*BpfAction)
		if !ok {
			t.Fatal("Action is the wrong type")
		}
	}
	if bpfAction.Fd != fd {
		t.Fatalf("Action Fd does not match %d != %d", bpfAction.Fd, fd)
	}
	if _, ok := u32.Actions[0].(*MirredAction); !ok {
		if _, ok := u32.Actions[1].(*MirredAction); !ok {
			t.Fatal("Action is the wrong type")
		}
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterU32ConnmarkAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Ingress)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}

	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Actions: []Action{
			&ConnmarkAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_PIPE,
				},
			},
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if len(u32.Actions) != 2 {
		t.Fatalf("Too few Actions in filter")
	}
	if u32.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}

	// actions can be returned in reverse order
	cma, ok := u32.Actions[0].(*ConnmarkAction)
	if !ok {
		cma, ok = u32.Actions[1].(*ConnmarkAction)
		if !ok {
			t.Fatal("Unable to find connmark action")
		}
	}

	if cma.Attrs().Action != TC_ACT_PIPE {
		t.Fatal("Connmark action isn't TC_ACT_PIPE")
	}

	mia, ok := u32.Actions[0].(*MirredAction)
	if !ok {
		mia, ok = u32.Actions[1].(*MirredAction)
		if !ok {
			t.Fatal("Unable to find mirred action")
		}
	}

	if mia.Attrs().Action != TC_ACT_STOLEN {
		t.Fatal("Mirred action isn't TC_ACT_STOLEN")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterU32CsumAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatalf("add link foo error: %v", err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatalf("add link foo error: %v", err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatalf("set foo link up error: %v", err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatalf("get qdisc error: %v", err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Actions: []Action{
			&CsumAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_PIPE,
				},
				UpdateFlags: TCA_CSUM_UPDATE_FLAG_TCP,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatalf("get filter error: %v", err)
	}

	if len(filters) != 1 {
		t.Fatalf("the count filters error, expect: 1, acutal: %d", len(filters))
	}

	ft, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if ft.LinkIndex != link.Attrs().Index {
		t.Fatal("link index error")
	}

	if len(ft.Actions) != 1 {
		t.Fatalf("filter has wrong number of actions, expect: 1, acutal: %d", len(filters))
	}

	csum, ok := ft.Actions[0].(*CsumAction)
	if !ok {
		t.Fatal("action is the wrong type")
	}

	if csum.Attrs().Action != TC_ACT_PIPE {
		t.Fatal("Csum action isn't TC_ACT_PIPE")
	}

	if csum.UpdateFlags != TCA_CSUM_UPDATE_FLAG_TCP {
		t.Fatalf("Csum action isn't TCA_CSUM_UPDATE_FLAG_TCP, got %d", csum.UpdateFlags)
	}

	if err := FilterDel(ft); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func setupLinkForTestWithQdisc(t *testing.T, linkName string) (Qdisc, Link) {
	if err := LinkAdd(&Ifb{LinkAttrs{Name: linkName}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName(linkName)
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	qdisc := &Clsact{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_CLSACT,
		},
	}

	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc", len(qdiscs))
	}
	if q, ok := qdiscs[0].(*Clsact); !ok || q.Type() != "clsact" {
		t.Fatal("qdisc is the wrong type")
	}
	return qdiscs[0], link
}

func TestFilterClsActBpfAddDel(t *testing.T) {
	t.Skipf("Fd does not match in ci")
	// This feature was added in kernel 4.5
	minKernelRequired(t, 4, 5)

	tearDown := setUpNetlinkTest(t)
	defer tearDown()

	qdisc, link := setupLinkForTestWithQdisc(t, "foo")
	filterattrs := FilterAttrs{
		LinkIndex: link.Attrs().Index,
		Parent:    HANDLE_MIN_EGRESS,
		Handle:    MakeHandle(0, 1),
		Protocol:  unix.ETH_P_ALL,
		Priority:  1,
	}
	fd, err := loadSimpleBpf(BPF_PROG_TYPE_SCHED_CLS, 1)
	if err != nil {
		t.Skipf("Loading bpf program failed: %s", err)
	}
	filter := &BpfFilter{
		FilterAttrs:  filterattrs,
		Fd:           fd,
		Name:         "simple",
		DirectAction: true,
	}
	if filter.Fd < 0 {
		t.Skipf("Failed to load bpf program")
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, HANDLE_MIN_EGRESS)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	bpf, ok := filters[0].(*BpfFilter)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if bpf.Fd != filter.Fd {
		t.Fatal("Filter Fd does not match")
	}
	if bpf.DirectAction != filter.DirectAction {
		t.Fatal("Filter DirectAction does not match")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, HANDLE_MIN_EGRESS)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterMatchAllAddDel(t *testing.T) {
	// This classifier was added in kernel 4.7
	minKernelRequired(t, 4, 7)

	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	_, link := setupLinkForTestWithQdisc(t, "foo")
	_, link2 := setupLinkForTestWithQdisc(t, "bar")
	filter := &MatchAll{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    HANDLE_MIN_EGRESS,
			Priority:  32000,
			Protocol:  unix.ETH_P_ALL,
		},
		Actions: []Action{
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      link2.Attrs().Index,
			},
		},
	}
	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, HANDLE_MIN_EGRESS)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	matchall, ok := filters[0].(*MatchAll)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if matchall.Priority != 32000 {
		t.Fatal("Filter priority does not match")
	}

	if len(matchall.Actions) != 1 {
		t.Fatal("Filter has no actions")
	}

	mirredAction, ok := matchall.Actions[0].(*MirredAction)
	if !ok {
		t.Fatal("Action does not match")
	}

	if mirredAction.Ifindex != link2.Attrs().Index {
		t.Fatal("Action ifindex does not match")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, HANDLE_MIN_EGRESS)
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

}

func TestFilterU32TunnelKeyAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	tunnelAct := NewTunnelKeyAction()
	tunnelAct.SrcAddr = net.IPv4(10, 10, 10, 1)
	tunnelAct.DstAddr = net.IPv4(10, 10, 10, 2)
	tunnelAct.KeyID = 0x01
	tunnelAct.Action = TCA_TUNNEL_KEY_SET
	tunnelAct.DestPort = 8472

	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Actions: []Action{
			tunnelAct,
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if len(u32.Actions) != 2 {
		t.Fatalf("Too few Actions in filter")
	}
	if u32.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}

	// actions can be returned in reverse order
	tun, ok := u32.Actions[0].(*TunnelKeyAction)
	if !ok {
		tun, ok = u32.Actions[1].(*TunnelKeyAction)
		if !ok {
			t.Fatal("Unable to find tunnel action")
		}
	}

	if tun.Attrs().Action != TC_ACT_PIPE {
		t.Fatal("TunnelKey action isn't TC_ACT_PIPE")
	}
	if !tun.SrcAddr.Equal(tunnelAct.SrcAddr) {
		t.Fatal("Action SrcAddr doesn't match")
	}
	if !tun.DstAddr.Equal(tunnelAct.DstAddr) {
		t.Fatal("Action DstAddr doesn't match")
	}
	if tun.KeyID != tunnelAct.KeyID {
		t.Fatal("Action KeyID doesn't match")
	}
	if tun.DestPort != tunnelAct.DestPort {
		t.Fatal("Action DestPort doesn't match")
	}
	if tun.Action != tunnelAct.Action {
		t.Fatal("Action doesn't match")
	}

	mia, ok := u32.Actions[0].(*MirredAction)
	if !ok {
		mia, ok = u32.Actions[1].(*MirredAction)
		if !ok {
			t.Fatal("Unable to find mirred action")
		}
	}

	if mia.Attrs().Action != TC_ACT_STOLEN {
		t.Fatal("Mirred action isn't TC_ACT_STOLEN")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterU32SkbEditAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	skbedit := NewSkbEditAction()
	ptype := uint16(unix.PACKET_HOST)
	skbedit.PType = &ptype
	priority := uint32(0xff)
	skbedit.Priority = &priority
	mark := uint32(0xfe)
	skbedit.Mark = &mark
	mask := uint32(0xff)
	skbedit.Mask = &mask
	mapping := uint16(0xf)
	skbedit.QueueMapping = &mapping

	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Actions: []Action{
			skbedit,
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if len(u32.Actions) != 2 {
		t.Fatalf("Too few Actions in filter")
	}
	if u32.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}

	// actions can be returned in reverse order
	edit, ok := u32.Actions[0].(*SkbEditAction)
	if !ok {
		edit, ok = u32.Actions[1].(*SkbEditAction)
		if !ok {
			t.Fatal("Unable to find tunnel action")
		}
	}

	if edit.Attrs().Action != TC_ACT_PIPE {
		t.Fatal("SkbEdit action isn't TC_ACT_PIPE")
	}
	if edit.PType == nil || *edit.PType != *skbedit.PType {
		t.Fatal("Action PType doesn't match")
	}
	if edit.QueueMapping == nil || *edit.QueueMapping != *skbedit.QueueMapping {
		t.Fatal("Action QueueMapping doesn't match")
	}
	if edit.Mark == nil || *edit.Mark != *skbedit.Mark {
		t.Fatal("Action Mark doesn't match")
	}
	if edit.Mask == nil || *edit.Mask != *skbedit.Mask {
		t.Fatal("Action Mask doesn't match")
	}
	if edit.Priority == nil || *edit.Priority != *skbedit.Priority {
		t.Fatal("Action Priority doesn't match")
	}

	mia, ok := u32.Actions[0].(*MirredAction)
	if !ok {
		mia, ok = u32.Actions[1].(*MirredAction)
		if !ok {
			t.Fatal("Unable to find mirred action")
		}
	}

	if mia.Attrs().Action != TC_ACT_STOLEN {
		t.Fatal("Mirred action isn't TC_ACT_STOLEN")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterU32LinkOption(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatalf("add link foo error: %v", err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatalf("add link foo error: %v", err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatalf("set foo link up error: %v", err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatalf("get qdisc error: %v", err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	htid := uint32(10)
	size := uint32(8)
	priority := uint16(200)
	u32Table := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    htid << 20,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  priority,
			Protocol:  unix.ETH_P_ALL,
		},
		Divisor: size,
	}
	if err := FilterAdd(u32Table); err != nil {
		t.Fatal(err)
	}

	u32 := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Handle:    1,
			Priority:  priority,
			Protocol:  unix.ETH_P_ALL,
		},
		Link: uint32(htid << 20),
		Sel: &TcU32Sel{
			Nkeys:    1,
			Flags:    TC_U32_TERMINAL | TC_U32_VAROFFSET,
			Hmask:    0x0000ff00,
			Hoff:     0,
			Offshift: 8,
			Keys: []TcU32Key{
				{
					Mask: 0,
					Val:  0,
					Off:  0,
				},
			},
		},
	}
	if err := FilterAdd(u32); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatalf("get filter error: %v", err)
	}

	if len(filters) != 1 {
		t.Fatalf("the count filters error, expect: 1, acutal: %d", len(filters))
	}

	ft, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if ft.LinkIndex != link.Attrs().Index {
		t.Fatal("link index error")
	}

	if ft.Link != htid<<20 {
		t.Fatal("hash table id error")
	}

	if ft.Priority != priority {
		t.Fatal("priority error")
	}

	if err := FilterDel(ft); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterFlowerAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	testMask := net.CIDRMask(24, 32)
	srcMac, err := net.ParseMAC("2C:54:91:88:C9:E3")
	if err != nil {
		t.Fatal(err)
	}
	destMac, err := net.ParseMAC("2C:54:91:88:C9:E5")
	if err != nil {
		t.Fatal(err)
	}

	ipproto := new(nl.IPProto)
	*ipproto = nl.IPPROTO_TCP

	filter := &Flower{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		DestIP:        net.ParseIP("1.0.0.1"),
		DestIPMask:    testMask,
		SrcIP:         net.ParseIP("2.0.0.1"),
		SrcIPMask:     testMask,
		EthType:       unix.ETH_P_IP,
		EncDestIP:     net.ParseIP("3.0.0.1"),
		EncDestIPMask: testMask,
		EncSrcIP:      net.ParseIP("4.0.0.1"),
		EncSrcIPMask:  testMask,
		EncDestPort:   8472,
		EncKeyId:      1234,
		SrcMac:        srcMac,
		DestMac:       destMac,
		IPProto:       ipproto,
		DestPort:      1111,
		SrcPort:       1111,
		Actions: []Action{
			&VlanAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_PIPE,
				},
				Action: TCA_VLAN_ACT_PUSH,
				VlanID: 1234,
			},
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
			&GenericAction{
				ActionAttrs: ActionAttrs{
					Action: getTcActGotoChain(),
				},
				Chain: 20,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	flower, ok := filters[0].(*Flower)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if filter.EthType != flower.EthType {
		t.Fatalf("Flower EthType doesn't match")
	}
	if !filter.DestIP.Equal(flower.DestIP) {
		t.Fatalf("Flower DestIP doesn't match")
	}
	if !filter.SrcIP.Equal(flower.SrcIP) {
		t.Fatalf("Flower SrcIP doesn't match")
	}

	if !reflect.DeepEqual(filter.DestIPMask, testMask) {
		t.Fatalf("Flower DestIPMask doesn't match")
	}
	if !reflect.DeepEqual(filter.SrcIPMask, testMask) {
		t.Fatalf("Flower SrcIPMask doesn't match")
	}

	if !filter.EncDestIP.Equal(flower.EncDestIP) {
		t.Fatalf("Flower EncDestIP doesn't match")
	}
	if !filter.EncSrcIP.Equal(flower.EncSrcIP) {
		t.Fatalf("Flower EncSrcIP doesn't match")
	}
	if !reflect.DeepEqual(filter.EncDestIPMask, testMask) {
		t.Fatalf("Flower EncDestIPMask doesn't match")
	}
	if !reflect.DeepEqual(filter.EncSrcIPMask, testMask) {
		t.Fatalf("Flower EncSrcIPMask doesn't match")
	}
	if filter.EncKeyId != flower.EncKeyId {
		t.Fatalf("Flower EncKeyId doesn't match")
	}
	if filter.EncDestPort != flower.EncDestPort {
		t.Fatalf("Flower EncDestPort doesn't match")
	}
	if flower.IPProto == nil || *filter.IPProto != *flower.IPProto {
		t.Fatalf("Flower IPProto doesn't match")
	}
	if filter.DestPort != flower.DestPort {
		t.Fatalf("Flower DestPort doesn't match")
	}
	if filter.SrcPort != flower.SrcPort {
		t.Fatalf("Flower SrcPort doesn't match")
	}
	if !(filter.SrcMac.String() == flower.SrcMac.String()) {
		t.Fatalf("Flower SrcMac doesn't match")
	}
	if !(filter.DestMac.String() == flower.DestMac.String()) {
		t.Fatalf("Flower DestMac doesn't match")
	}

	vla, ok := flower.Actions[0].(*VlanAction)
	if !ok {
		t.Fatal("Unable to find vlan action")
	}

	if vla.Attrs().Action != TC_ACT_PIPE {
		t.Fatal("Vlan action isn't TC_ACT_PIPE")
	}

	if vla.Action != TCA_VLAN_ACT_PUSH {
		t.Fatal("Second Vlan action isn't push")
	}

	if vla.VlanID != 1234 {
		t.Fatal("Second Vlan action vlanId isn't correct")
	}

	mia, ok := flower.Actions[1].(*MirredAction)
	if !ok {
		t.Fatal("Unable to find mirred action")
	}

	if mia.Attrs().Action != TC_ACT_STOLEN {
		t.Fatal("Mirred action isn't TC_ACT_STOLEN")
	}

	if mia.Timestamp == nil || mia.Timestamp.Installed == 0 {
		t.Fatal("Incorrect mirred action timestamp")
	}

	if mia.Statistics == nil {
		t.Fatal("Incorrect mirred action stats")
	}

	ga, ok := flower.Actions[2].(*GenericAction)
	if !ok {
		t.Fatal("Unable to find generic action")
	}

	if ga.Attrs().Action != getTcActGotoChain() {
		t.Fatal("Generic action isn't TC_ACT_GOTO_CHAIN")
	}

	if ga.Timestamp == nil || ga.Timestamp.Installed == 0 {
		t.Fatal("Incorrect generic action timestamp")
	}

	if ga.Statistics == nil {
		t.Fatal("Incorrect generic action stats")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	filter = &Flower{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_8021Q,
		},
		EthType: unix.ETH_P_8021Q,
		VlanId:  2046,
		Actions: []Action{
			&VlanAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_PIPE,
				},
				Action: TCA_VLAN_ACT_POP,
			},
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	flower, ok = filters[0].(*Flower)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if filter.VlanId != flower.VlanId {
		t.Fatalf("Flower VlanId doesn't match")
	}

	vla, ok = flower.Actions[0].(*VlanAction)
	if !ok {
		t.Fatal("Unable to find vlan action")
	}

	if vla.Attrs().Action != TC_ACT_PIPE {
		t.Fatal("Vlan action isn't TC_ACT_PIPE")
	}

	if vla.Action != TCA_VLAN_ACT_POP {
		t.Fatal("First Vlan action isn't pop")
	}

	mia, ok = flower.Actions[1].(*MirredAction)
	if !ok {
		t.Fatal("Unable to find mirred action")
	}

	if mia.Attrs().Action != TC_ACT_STOLEN {
		t.Fatal("Mirred action isn't TC_ACT_STOLEN")
	}

	if mia.Timestamp == nil || mia.Timestamp.Installed == 0 {
		t.Fatal("Incorrect mirred action timestamp")
	}

	if mia.Statistics == nil {
		t.Fatal("Incorrect mirred action stats")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	classId := MakeHandle(1, 101)

	filter = &Flower{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},

		EthType:         unix.ETH_P_IP,
		IPProto:         ipproto,
		ClassId:         classId,
		SrcPortRangeMin: 1000,
		SrcPortRangeMax: 2000,
	}
	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	flower, ok = filters[0].(*Flower)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}
	if filter.ClassId != flower.ClassId {
		t.Fatalf("Flower ClassId doesn't match")
	}
	if filter.SrcPortRangeMin != flower.SrcPortRangeMin {
		t.Fatalf("Flower SrcPortRangeMin doesn't match")
	}
	if filter.SrcPortRangeMax != flower.SrcPortRangeMax {
		t.Fatalf("Flower SrcPortRangeMax doesn't match")
	}
	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterIPv6FlowerPedit(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	testMask := net.CIDRMask(64, 128)

	ipproto := new(nl.IPProto)
	*ipproto = nl.IPPROTO_TCP

	filter := &Flower{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		DestIP:     net.ParseIP("ffff::fff1"),
		DestIPMask: testMask,
		EthType:    unix.ETH_P_IPV6,
		IPProto:    ipproto,
		DestPort:   6666,
		Actions:    []Action{},
	}

	peditAction := NewPeditAction()
	peditAction.Proto = uint8(nl.IPPROTO_TCP)
	peditAction.SrcPort = 7777
	peditAction.SrcIP = net.ParseIP("ffff::fff2")
	filter.Actions = append(filter.Actions, peditAction)

	miaAction := &MirredAction{
		ActionAttrs: ActionAttrs{
			Action: TC_ACT_REDIRECT,
		},
		MirredAction: TCA_EGRESS_REDIR,
		Ifindex:      redir.Attrs().Index,
	}
	filter.Actions = append(filter.Actions, miaAction)

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	flower, ok := filters[0].(*Flower)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if filter.EthType != flower.EthType {
		t.Fatalf("Flower EthType doesn't match")
	}
	if !filter.DestIP.Equal(flower.DestIP) {
		t.Fatalf("Flower DestIP doesn't match")
	}

	if !reflect.DeepEqual(filter.DestIPMask, testMask) {
		t.Fatalf("Flower DestIPMask doesn't match")
	}

	if flower.IPProto == nil || *filter.IPProto != *flower.IPProto {
		t.Fatalf("Flower IPProto doesn't match")
	}
	if filter.DestPort != flower.DestPort {
		t.Fatalf("Flower DestPort doesn't match")
	}

	_, ok = flower.Actions[0].(*PeditAction)
	if !ok {
		t.Fatal("Unable to find pedit action")
	}

	_, ok = flower.Actions[1].(*MirredAction)
	if !ok {
		t.Fatal("Unable to find mirred action")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterU32PoliceAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	const (
		policeRate     = 0x40000000 // 1 Gbps
		policeBurst    = 0x19000    // 100 KB
		policePeakRate = 0x4000     // 16 Kbps
	)

	police := NewPoliceAction()
	police.Rate = policeRate
	police.PeakRate = policePeakRate
	police.Burst = policeBurst
	police.ExceedAction = TC_POLICE_SHOT
	police.NotExceedAction = TC_POLICE_UNSPEC

	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Actions: []Action{
			police,
			&MirredAction{
				ActionAttrs: ActionAttrs{
					Action: TC_ACT_STOLEN,
				},
				MirredAction: TCA_EGRESS_REDIR,
				Ifindex:      redir.Attrs().Index,
			},
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if len(u32.Actions) != 2 {
		t.Fatalf("Too few Actions in filter")
	}
	if u32.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}

	// actions can be returned in reverse order
	p, ok := u32.Actions[0].(*PoliceAction)
	if !ok {
		p, ok = u32.Actions[1].(*PoliceAction)
		if !ok {
			t.Fatal("Unable to find police action")
		}
	}

	if p.ExceedAction != TC_POLICE_SHOT {
		t.Fatal("Police ExceedAction isn't TC_POLICE_SHOT")
	}

	if p.NotExceedAction != TC_POLICE_UNSPEC {
		t.Fatal("Police NotExceedAction isn't TC_POLICE_UNSPEC")
	}

	if p.Rate != policeRate {
		t.Fatal("Action Rate doesn't match")
	}

	if p.PeakRate != policePeakRate {
		t.Fatal("Action PeakRate doesn't match")
	}

	if p.LinkLayer != nl.LINKLAYER_ETHERNET {
		t.Fatal("Action LinkLayer doesn't match")
	}

	mia, ok := u32.Actions[0].(*MirredAction)
	if !ok {
		mia, ok = u32.Actions[1].(*MirredAction)
		if !ok {
			t.Fatal("Unable to find mirred action")
		}
	}

	if mia.Attrs().Action != TC_ACT_STOLEN {
		t.Fatal("Mirred action isn't TC_ACT_STOLEN")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterU32DirectPoliceAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}

	const (
		policeRate     = 0x40000000 // 1 Gbps
		policeBurst    = 0x19000    // 100 KB
		policePeakRate = 0x4000     // 16 Kbps
	)

	police := NewPoliceAction()
	police.Rate = policeRate
	police.PeakRate = policePeakRate
	police.Burst = policeBurst
	police.ExceedAction = TC_POLICE_SHOT
	police.NotExceedAction = TC_POLICE_UNSPEC

	classId := MakeHandle(1, 1)
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Police:  police,
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	u32, ok := filters[0].(*U32)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if u32.Police == nil {
		t.Fatalf("No police in filter")
	}

	if u32.Police.Rate != policeRate {
		t.Fatal("Filter Rate doesn't match")
	}

	if u32.Police.PeakRate != policePeakRate {
		t.Fatal("Filter PeakRate doesn't match")
	}

	if u32.Police.LinkLayer != nl.LINKLAYER_ETHERNET {
		t.Fatal("Filter LinkLayer doesn't match")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterChainAddDel(t *testing.T) {
	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "bar"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}
	redir, err := LinkByName("bar")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(redir); err != nil {
		t.Fatal(err)
	}
	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 1 {
		t.Fatal("Failed to add qdisc")
	}
	_, ok := qdiscs[0].(*Ingress)
	if !ok {
		t.Fatal("Qdisc is the wrong type")
	}
	classId := MakeHandle(1, 1)
	chainVal := new(uint32)
	*chainVal = 20
	filter := &U32{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_IP,
			Chain:     chainVal,
		},
		RedirIndex: redir.Attrs().Index,
		ClassId:    classId,
	}
	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}
	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	filterChain := filters[0].Attrs().Chain
	if filterChain != nil && *filterChain != *chainVal {
		t.Fatalf("Chain of the filter is the wrong value")
	}
	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}
	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}
	if len(qdiscs) != 0 {
		t.Fatal("Failed to remove qdisc")
	}
}

func TestFilterSampleAddDel(t *testing.T) {
	minKernelRequired(t, 4, 11)
	if _, err := GenlFamilyGet("psample"); err != nil {
		t.Skip("psample genetlink family unavailable - is CONFIG_PSAMPLE enabled?")
	}

	tearDown := setUpNetlinkTest(t)
	defer tearDown()
	if err := LinkAdd(&Ifb{LinkAttrs{Name: "foo"}}); err != nil {
		t.Fatal(err)
	}
	link, err := LinkByName("foo")
	if err != nil {
		t.Fatal(err)
	}
	if err := LinkSetUp(link); err != nil {
		t.Fatal(err)
	}

	qdisc := &Ingress{
		QdiscAttrs: QdiscAttrs{
			LinkIndex: link.Attrs().Index,
			Handle:    MakeHandle(0xffff, 0),
			Parent:    HANDLE_INGRESS,
		},
	}
	if err := QdiscAdd(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err := SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Qdisc is the wrong type")
	}

	sample := NewSampleAction()
	sample.Group = 7
	sample.Rate = 12
	sample.TruncSize = 200

	classId := MakeHandle(1, 1)
	filter := &MatchAll{
		FilterAttrs: FilterAttrs{
			LinkIndex: link.Attrs().Index,
			Parent:    MakeHandle(0xffff, 0),
			Priority:  1,
			Protocol:  unix.ETH_P_ALL,
		},
		ClassId: classId,
		Actions: []Action{
			sample,
		},
	}

	if err := FilterAdd(filter); err != nil {
		t.Fatal(err)
	}

	filters, err := FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 1 {
		t.Fatal("Failed to add filter")
	}
	mf, ok := filters[0].(*MatchAll)
	if !ok {
		t.Fatal("Filter is the wrong type")
	}

	if len(mf.Actions) < 1 {
		t.Fatalf("Too few Actions in filter")
	}
	if mf.ClassId != classId {
		t.Fatalf("ClassId of the filter is the wrong value")
	}

	lsample, ok := mf.Actions[0].(*SampleAction)
	if !ok {
		t.Fatal("Unable to find sample action")
	}
	if lsample.Group != sample.Group {
		t.Fatalf("Inconsistent sample action group")
	}
	if lsample.Rate != sample.Rate {
		t.Fatalf("Inconsistent sample action rate")
	}
	if lsample.TruncSize != sample.TruncSize {
		t.Fatalf("Inconsistent sample truncation size")
	}

	if err := FilterDel(filter); err != nil {
		t.Fatal(err)
	}
	filters, err = FilterList(link, MakeHandle(0xffff, 0))
	if err != nil {
		t.Fatal(err)
	}
	if len(filters) != 0 {
		t.Fatal("Failed to remove filter")
	}

	if err := QdiscDel(qdisc); err != nil {
		t.Fatal(err)
	}
	qdiscs, err = SafeQdiscList(link)
	if err != nil {
		t.Fatal(err)
	}

	found = false
	for _, v := range qdiscs {
		if _, ok := v.(*Ingress); ok {
			found = true
			break
		}
	}
	if found {
		t.Fatal("Failed to remove qdisc")
	}
}
