package sample

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeProc writes a /proc of the process 4242 and a container's cgroup
// under a temporary directory: CLK_TCK 100, the process started at tick
// 123,456 (1,234.56 s after boot), the uptime 5,000.25 s, btime a whole
// second that /proc/uptime's hundredths disagree with.
func fakeProc(t *testing.T) (proc, cgroup string) {
	t.Helper()
	dir := t.TempDir()
	proc, cgroup = filepath.Join(dir, "proc"), CgroupOf(filepath.Join(dir, "system.slice"), "c0ffee")
	files := map[string]string{
		filepath.Join(proc, "4242", "status"): "Name:\tnervewiki\nVmPeak:\t  999999 kB\nVmHWM:\t  204800 kB\nVmRSS:\t  102400 kB\nThreads:\t12\n",
		// The command holds a parenthesis and spaces: the fields count from
		// its last ')'. utime 250 and stime 75 ticks, starttime 123456.
		filepath.Join(proc, "4242", "stat"):     "4242 (nerve (wiki) x) S 1 4242 4242 0 -1 4194560 100 0 0 0 250 75 0 0 20 0 12 0 123456 1000000 25600 18446744073709551615\n",
		filepath.Join(proc, "uptime"):           "5000.25 39000.10\n",
		filepath.Join(proc, "stat"):             "cpu  1 2 3 4\nbtime 1795000000\n",
		filepath.Join(cgroup, "memory.current"): "52428800\n",
		filepath.Join(cgroup, "cpu.stat"):       "usage_usec 1234567\nuser_usec 1000000\nsystem_usec 234567\n",
		filepath.Join(cgroup, "memory.peak"):    "73400320\n",
		filepath.Join(cgroup, "memory.stat"):    "anon 31457280\nfile 20971520\nkernel 1048576\nshmem 4194304\nfile_mapped 1024\n",
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return proc, cgroup
}

// The sampler reads kB as bytes, the ticks by CLK_TCK, the process's
// start from the uptime to the 0.01 s, the container's final values from
// memory.peak and memory.stat, and fails without the cgroup (design
// 3.4.2, 7.2's 27 to 30).
func TestTheSamplerReadsProcAndTheCgroup(t *testing.T) {
	proc, cgroup := fakeProc(t)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	s, err := Open(proc, cgroup, 4242, 100, now)
	if err != nil {
		t.Fatal(err)
	}
	// Its age: 5,000.25 s less 1,234.56 s, 3,765.69 s.
	if want := now.Add(-3765690 * time.Millisecond); !s.Started().Equal(want) {
		t.Errorf("started %v, want %v (now less 3,765.69 s)", s.Started(), want)
	}
	if btime := time.Unix(1795000000+1234, 560_000_000); s.Started().Equal(btime) {
		t.Error("the start is btime's")
	}
	r, err := s.Sample(now)
	if err != nil {
		t.Fatal(err)
	}
	want := Row{At: now, RSS: 102400 * 1024, HWM: 204800 * 1024, UserNS: 2_500_000_000, SystemNS: 750_000_000,
		MemoryCurrent: 52428800, ContainerCPUMicro: 1234567, Anon: 31457280, Shmem: 4194304, File: 20971520}
	if r != want {
		t.Errorf("sample %+v, want %+v", r, want)
	}
	if line := r.CSV(); line != "1794916800000000000,104857600,209715200,2500000000,750000000,52428800,1234567,31457280,4194304,20971520" {
		t.Errorf("csv %q", line)
	}
	f, err := s.Final()
	if err != nil {
		t.Fatal(err)
	}
	if f != (Final{Peak: 73400320, Anon: 31457280, File: 20971520, Shmem: 4194304}) {
		t.Errorf("final %+v, want memory.peak and memory.stat's", f)
	}
	// Another CLK_TCK: the ticks are read by it.
	s250, err := Open(proc, cgroup, 4242, 250, now)
	if err != nil {
		t.Fatal(err)
	}
	if r, err := s250.Sample(now); err != nil || r.UserNS != 1_000_000_000 || r.SystemNS != 300_000_000 {
		t.Errorf("at 250 ticks a second: %+v, %v", r, err)
	}
	if want := now.Add(-(500025 - 123456*100/250) * 10 * time.Millisecond); !s250.Started().Equal(want) {
		t.Errorf("at 250 ticks a second started %v, want %v", s250.Started(), want)
	}
	for _, bad := range []struct {
		name   string
		cgroup string
		pid    int
		tick   int64
	}{
		{"no cgroup", filepath.Join(filepath.Dir(cgroup), "docker-gone.scope"), 4242, 100},
		{"no process", cgroup, 4343, 100},
		{"no CLK_TCK", cgroup, 4242, 0},
	} {
		if _, err := Open(proc, bad.cgroup, bad.pid, bad.tick, now); err == nil {
			t.Errorf("%s: opened", bad.name)
		}
	}
	// The cgroup gone after it opened: a sample fails, not zero.
	if err := os.Remove(filepath.Join(cgroup, "memory.current")); err != nil {
		t.Fatal(err)
	}
	if r, err := s.Sample(now); err == nil {
		t.Errorf("without memory.current: %+v", r)
	}
}

// Run writes the start, the header, the rows and, once ctx ends, the
// final values.
func TestTheSamplerRunsToItsFinalLine(t *testing.T) {
	proc, cgroup := fakeProc(t)
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	s, err := Open(proc, cgroup, 4242, 100, now)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	rows := 0
	// The second row's anon and shmem the largest, the third's file, which
	// the final values read again.
	stats := []string{"anon 1000\nfile 2000\nshmem 300\n", "anon 9000\nfile 2500\nshmem 700\n", "anon 4000\nfile 8000\nshmem 500\n"}
	clock := func() time.Time {
		if err := os.WriteFile(filepath.Join(cgroup, "memory.stat"), []byte(stats[rows]), 0o644); err != nil {
			t.Fatal(err)
		}
		if rows++; rows == 3 {
			cancel()
		}
		return now.Add(time.Duration(rows) * 100 * time.Millisecond)
	}
	var out bytes.Buffer
	if err := s.Run(ctx, &out, time.Millisecond, clock); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 6 || lines[0] != "# pid=4242 clk_tck=100 started_unix_ns=1794913034310000000" || lines[1] != Header ||
		!strings.HasPrefix(lines[2], "1794916800100000000,") || !strings.HasSuffix(lines[2], ",1000,300,2000") ||
		!strings.HasSuffix(lines[4], ",4000,500,8000") ||
		lines[5] != "# final memory_peak=73400320 anon=4000 file=8000 shmem=500 max_anon=9000 max_shmem=700 max_file=8000" {
		t.Errorf("run wrote:\n%s", out.String())
	}
}
