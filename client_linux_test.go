//+build linux

package taskstats

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"unsafe"

	"github.com/google/go-cmp/cmp"
	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/genetlink/genltest"
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

func TestLinuxClientCGroupStatsBadMessages(t *testing.T) {
	f, done := tempFile(t)
	defer done()

	tests := []struct {
		name string
		msgs []genetlink.Message
	}{
		{
			name: "no messages",
			msgs: []genetlink.Message{},
		},
		{
			name: "two messages",
			msgs: []genetlink.Message{{}, {}},
		},
		{
			name: "incorrect cgroupstats size",
			msgs: []genetlink.Message{{
				Data: mustMarshalAttributes([]netlink.Attribute{{
					Type: unix.CGROUPSTATS_TYPE_CGROUP_STATS,
					Data: []byte{0xff},
				}}),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(t, func(_ genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
				return tt.msgs, nil
			})
			defer c.Close()

			_, err := c.CGroupStats(f)
			if err == nil {
				t.Fatal("an error was expected, but none occurred")
			}
		})
	}
}

func TestLinuxClientCGroupStatsIsNotExist(t *testing.T) {
	tests := []struct {
		name       string
		msg        genetlink.Message
		createFile bool
	}{
		{
			name: "no file",
		},
		{
			name:       "no attributes",
			msg:        genetlink.Message{},
			createFile: true,
		},
		{
			name: "no aggr+pid",
			msg: genetlink.Message{
				Data: mustMarshalAttributes([]netlink.Attribute{{
					Type: unix.TASKSTATS_TYPE_NULL,
				}}),
			},
			createFile: true,
		},
		{
			name: "no stats",
			msg: genetlink.Message{
				Data: mustMarshalAttributes([]netlink.Attribute{{
					// Wrong type for cgroup stats.
					Type: unix.TASKSTATS_TYPE_AGGR_PID,
				}}),
			},
			createFile: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Only create the file when requested, so we can also exercise the
			// case where the file doesn't exist.
			var f string
			if tt.createFile {
				file, done := tempFile(t)
				defer done()
				f = file
			}

			c := testClient(t, func(_ genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
				return []genetlink.Message{tt.msg}, nil
			})
			defer c.Close()

			_, err := c.CGroupStats(f)
			if !os.IsNotExist(err) {
				t.Fatalf("expected is not exist, but got: %v", err)
			}
		})
	}
}

func TestLinuxClientCGroupStatsOK(t *testing.T) {
	f, done := tempFile(t)
	defer done()

	stats := unix.CGroupStats{
		Sleeping:        1,
		Running:         2,
		Stopped:         3,
		Uninterruptible: 4,
		Io_wait:         5,
	}

	fn := func(gm genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
		attrs, err := netlink.UnmarshalAttributes(gm.Data)
		if err != nil {
			t.Fatalf("failed to unmarshal netlink attributes: %v", err)
		}

		if l := len(attrs); l != 1 {
			t.Fatalf("unexpected number of attributes: %d", l)
		}

		if diff := cmp.Diff(unix.CGROUPSTATS_CMD_ATTR_FD, int(attrs[0].Type)); diff != "" {
			t.Fatalf("unexpected netlink attribute type (-want +got):\n%s", diff)
		}

		// Cast unix.CGroupStats structure into a byte array with the correct size.
		b := *(*[sizeofCGroupStats]byte)(unsafe.Pointer(&stats))

		return []genetlink.Message{{
			Data: mustMarshalAttributes([]netlink.Attribute{{
				Type: unix.CGROUPSTATS_TYPE_CGROUP_STATS,
				Data: b[:],
			}}),
		}}, nil
	}

	c := testClient(t, checkRequest(unix.CGROUPSTATS_CMD_GET, netlink.HeaderFlagsRequest, fn))
	defer c.Close()

	newStats, err := c.CGroupStats(f)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	cstats, err := parseCGroupStats(stats)
	if err != nil {
		t.Fatalf("failed to parse cgroup stats: %v", err)
	}

	if diff := cmp.Diff(cstats, newStats); diff != "" {
		t.Fatalf("unexpected cgroupstats structure (-want +got):\n%s", diff)
	}
}

func TestLinuxClientPIDBadMessages(t *testing.T) {
	tests := []struct {
		name string
		msgs []genetlink.Message
	}{
		{
			name: "no messages",
			msgs: []genetlink.Message{},
		},
		{
			name: "two messages",
			msgs: []genetlink.Message{{}, {}},
		},
		{
			name: "incorrect taskstats size",
			msgs: []genetlink.Message{{
				Data: mustMarshalAttributes([]netlink.Attribute{{
					Type: unix.TASKSTATS_TYPE_AGGR_PID,
					Data: mustMarshalAttributes([]netlink.Attribute{{
						Type: unix.TASKSTATS_TYPE_STATS,
						Data: []byte{0xff},
					}}),
				}}),
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(t, func(_ genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
				return tt.msgs, nil
			})
			defer c.Close()

			_, err := c.PID(1)
			if err == nil {
				t.Fatal("an error was expected, but none occurred")
			}
		})
	}
}

func TestLinuxClientPIDIsNotExist(t *testing.T) {
	tests := []struct {
		name string
		msg  genetlink.Message
	}{
		{
			name: "no attributes",
			msg:  genetlink.Message{},
		},
		{
			name: "no aggr+pid",
			msg: genetlink.Message{
				Data: mustMarshalAttributes([]netlink.Attribute{{
					Type: unix.TASKSTATS_TYPE_NULL,
				}}),
			},
		},
		{
			name: "no stats",
			msg: genetlink.Message{
				Data: mustMarshalAttributes([]netlink.Attribute{{
					Type: unix.TASKSTATS_TYPE_AGGR_PID,
					Data: mustMarshalAttributes([]netlink.Attribute{{
						Type: unix.TASKSTATS_TYPE_NULL,
					}}),
				}}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testClient(t, func(_ genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
				return []genetlink.Message{tt.msg}, nil
			})
			defer c.Close()

			_, err := c.PID(1)
			if !os.IsNotExist(err) {
				t.Fatalf("expected is not exist, but got: %v", err)
			}
		})
	}
}

func TestLinuxClientPIDOK(t *testing.T) {
	pid := os.Getpid()

	stats := unix.Taskstats{
		Version: unix.TASKSTATS_VERSION,
		Ac_pid:  uint32(pid),
	}

	fn := func(_ genetlink.Message, _ netlink.Message) ([]genetlink.Message, error) {
		// Cast unix.Taskstats structure into a byte array with the correct size.
		b := *(*[sizeofTaskstats]byte)(unsafe.Pointer(&stats))

		return []genetlink.Message{{
			Data: mustMarshalAttributes([]netlink.Attribute{{
				Type: unix.TASKSTATS_TYPE_AGGR_PID,
				Data: mustMarshalAttributes([]netlink.Attribute{{
					Type: unix.TASKSTATS_TYPE_STATS,
					Data: b[:],
				}}),
			}}),
		}}, nil
	}

	c := testClient(t, checkRequest(unix.TASKSTATS_CMD_GET, netlink.HeaderFlagsRequest, fn))
	defer c.Close()

	newStats, err := c.PID(pid)
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}

	tstats := Stats(stats)

	if diff := cmp.Diff(&tstats, newStats); diff != "" {
		t.Fatalf("unexpected taskstats structure (-want +got):\n%s", diff)
	}
}

func checkRequest(command uint8, flags netlink.HeaderFlags, fn genltest.Func) genltest.Func {
	return func(greq genetlink.Message, nreq netlink.Message) ([]genetlink.Message, error) {
		if want, got := command, greq.Header.Command; command != 0 && want != got {
			return nil, fmt.Errorf("unexpected generic netlink header command: %d, want: %d", got, want)
		}

		if want, got := flags, nreq.Header.Flags; flags != 0 && want != got {
			return nil, fmt.Errorf("unexpected generic netlink header command: %s, want: %s", got, want)
		}

		return fn(greq, nreq)
	}
}

func testClient(t *testing.T, fn genltest.Func) *client {
	family := genetlink.Family{
		ID:      20,
		Version: unix.TASKSTATS_GENL_VERSION,
		Name:    unix.TASKSTATS_GENL_NAME,
	}

	conn := genltest.Dial(genltest.ServeFamily(family, fn))

	c, err := initClient(conn)
	if err != nil {
		t.Fatalf("failed to open client: %v", err)
	}

	return c
}

func tempFile(t *testing.T) (string, func()) {
	f, err := ioutil.TempFile(os.TempDir(), "taskstats-test")
	if err != nil {
		t.Fatalf("failed to create temporary file: %v", err)
	}
	_ = f.Close()

	return f.Name(), func() {
		if err := os.Remove(f.Name()); err != nil {
			t.Fatalf("failed to remove temporary file: %v", err)
		}
	}
}

func mustMarshalAttributes(attrs []netlink.Attribute) []byte {
	b, err := netlink.MarshalAttributes(attrs)
	if err != nil {
		panic(fmt.Sprintf("failed to marshal attributes: %v", err))
	}

	return b
}
