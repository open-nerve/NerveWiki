// Package sample reads what a measurement keeps of a process and a
// container (M12/P4 design 3.4.2): a serve's resident memory, its peak and
// its CPU from /proc, a PostgreSQL container's memory and CPU from its
// cgroup, and, in gctrace.go, the Go runtime's gc trace from the serve's
// log. The paths are given, so that a test reads files it wrote.
package sample

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Sampler reads a process's /proc and a container's cgroup.
type Sampler struct {
	proc, cgroup string // /proc and the container's own cgroup directory
	pid          int
	tick         int64     // CLK_TCK, the clock ticks of a second
	started      time.Time // the process's start, to 0.01 s
}

// Row is one sample: its time, the process's VmRSS and VmHWM in bytes, its
// user and system CPU in nanoseconds, the container's memory.current in
// bytes and cpu.stat's usage_usec, and memory.stat's anon, shmem and file:
// memory.current and memory.peak are mostly the page cache of a
// PostgreSQL's (review B's Q1). file holds shmem: the page cache the
// kernel can take back is about file - shmem, what it cannot is anon +
// shmem; the three do not add up (review B's N7).
type Row struct {
	At                time.Time
	RSS, HWM          uint64
	UserNS, SystemNS  int64
	MemoryCurrent     uint64
	ContainerCPUMicro uint64
	Anon, Shmem, File uint64
}

// Final is what the container's cgroup keeps of the run, read once at its
// end: memory.peak, and memory.stat's anon, file and shmem, in bytes; and,
// by Run, the largest anon, shmem and file of its rows.
type Final struct {
	Peak, Anon, File, Shmem    uint64
	MaxAnon, MaxShmem, MaxFile uint64
}

// CgroupOf is a docker container's cgroup directory under the systemd
// cgroup driver, under parent (/sys/fs/cgroup/system.slice).
func CgroupOf(parent, container string) string {
	return filepath.Join(parent, "docker-"+container+".scope")
}

// ClockTicks is getconf CLK_TCK's answer, the ticks /proc counts a second
// in.
func ClockTicks(ctx context.Context) (int64, error) {
	out, err := exec.CommandContext(ctx, "getconf", "CLK_TCK").Output()
	if err != nil {
		return 0, fmt.Errorf("getconf CLK_TCK: %w", err)
	}
	return strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
}

// Open is a sampler of the process pid under proc and of the container
// whose cgroup directory is cgroup, with tick clock ticks a second: an
// error unless tick is positive and the process and the cgroup directory
// are there (the cgroup is never read as zero, review A's N5). It reads
// the process's start as of now: now less the process's age, /proc/uptime
// less its starttime's ticks, to the 0.01 s /proc/uptime has (review A's
// M5: /proc/stat's btime has whole seconds alone).
func Open(proc, cgroup string, pid int, tick int64, now time.Time) (*Sampler, error) {
	if tick <= 0 {
		return nil, fmt.Errorf("CLK_TCK is %d", tick)
	}
	if info, err := os.Stat(cgroup); err != nil {
		return nil, fmt.Errorf("the container's cgroup: %w", err)
	} else if !info.IsDir() {
		return nil, fmt.Errorf("the container's cgroup %s is no directory", cgroup)
	}
	s := &Sampler{proc: proc, cgroup: cgroup, pid: pid, tick: tick}
	stat, err := s.stat()
	if err != nil {
		return nil, err
	}
	uptime, err := os.ReadFile(filepath.Join(proc, "uptime"))
	if err != nil {
		return nil, err
	}
	up, err := centiseconds(strings.Fields(string(uptime)))
	if err != nil {
		return nil, fmt.Errorf("%s/uptime: %w", proc, err)
	}
	age := up - stat.start*100/tick
	if age < 0 {
		return nil, fmt.Errorf("the process started %d cs after the uptime %d cs", -age, up)
	}
	s.started = now.Add(-time.Duration(age) * 10 * time.Millisecond)
	return s, nil
}

// centiseconds is the first of fields, seconds with two decimals, in
// hundredths of a second.
func centiseconds(fields []string) (int64, error) {
	if len(fields) == 0 {
		return 0, errors.New("empty")
	}
	whole, frac, ok := strings.Cut(fields[0], ".")
	if !ok || len(frac) != 2 {
		return 0, fmt.Errorf("%q is no seconds to 0.01", fields[0])
	}
	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, err
	}
	f, err := strconv.ParseInt(frac, 10, 64)
	if err != nil {
		return 0, err
	}
	return w*100 + f, nil
}

// Started is the process's start on the wall clock, to 0.01 s.
func (s *Sampler) Started() time.Time { return s.started }

// procStat is what /proc/<pid>/stat gives: user and system time and the
// start, in clock ticks.
type procStat struct{ user, system, start int64 }

// stat reads /proc/<pid>/stat: its fields after the command's closing
// parenthesis, the command itself holding any; utime, stime and starttime
// are the 14th, 15th and 22nd (proc(5)).
func (s *Sampler) stat() (procStat, error) {
	b, err := os.ReadFile(filepath.Join(s.proc, strconv.Itoa(s.pid), "stat"))
	if err != nil {
		return procStat{}, err
	}
	end := bytes.LastIndexByte(b, ')')
	if end < 0 {
		return procStat{}, fmt.Errorf("/proc/%d/stat has no command", s.pid)
	}
	fields := strings.Fields(string(b[end+1:])) // the 3rd field on
	field := func(n int) (int64, error) {
		if n-3 >= len(fields) {
			return 0, fmt.Errorf("/proc/%d/stat has %d fields, not %d", s.pid, len(fields)+2, n)
		}
		return strconv.ParseInt(fields[n-3], 10, 64)
	}
	var st procStat
	for _, f := range []struct {
		n  int
		to *int64
	}{{14, &st.user}, {15, &st.system}, {22, &st.start}} {
		if *f.to, err = field(f.n); err != nil {
			return procStat{}, err
		}
	}
	return st, nil
}

// keyed reads a file of lines of a key and a number, and answers the
// numbers of keys, each times by; a key not there is an error.
func keyed(path string, by uint64, keys ...string) ([]uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	found := map[string]uint64{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		f := strings.Fields(strings.Replace(sc.Text(), ":", " ", 1))
		if len(f) < 2 {
			continue
		}
		n, err := strconv.ParseUint(f[1], 10, 64)
		if err != nil {
			continue
		}
		found[f[0]] = n * by
	}
	out := make([]uint64, len(keys))
	for i, k := range keys {
		n, ok := found[k]
		if !ok {
			return nil, fmt.Errorf("%s has no %s", path, k)
		}
		out[i] = n
	}
	return out, nil
}

// number reads a file of one number.
func number(path string) (uint64, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
}

// Sample reads a row at at.
func (s *Sampler) Sample(at time.Time) (Row, error) {
	r := Row{At: at}
	mem, err := keyed(filepath.Join(s.proc, strconv.Itoa(s.pid), "status"), 1024, "VmRSS", "VmHWM")
	if err != nil {
		return Row{}, err
	}
	r.RSS, r.HWM = mem[0], mem[1]
	st, err := s.stat()
	if err != nil {
		return Row{}, err
	}
	r.UserNS, r.SystemNS = st.user*int64(time.Second)/s.tick, st.system*int64(time.Second)/s.tick
	if r.MemoryCurrent, err = number(filepath.Join(s.cgroup, "memory.current")); err != nil {
		return Row{}, err
	}
	cpu, err := keyed(filepath.Join(s.cgroup, "cpu.stat"), 1, "usage_usec")
	if err != nil {
		return Row{}, err
	}
	r.ContainerCPUMicro = cpu[0]
	kinds, err := keyed(filepath.Join(s.cgroup, "memory.stat"), 1, "anon", "shmem", "file")
	if err != nil {
		return Row{}, err
	}
	r.Anon, r.Shmem, r.File = kinds[0], kinds[1], kinds[2]
	return r, nil
}

// Final reads the container's peak and its memory's kinds.
func (s *Sampler) Final() (Final, error) {
	peak, err := number(filepath.Join(s.cgroup, "memory.peak"))
	if err != nil {
		return Final{}, err
	}
	kinds, err := keyed(filepath.Join(s.cgroup, "memory.stat"), 1, "anon", "file", "shmem")
	if err != nil {
		return Final{}, err
	}
	return Final{Peak: peak, Anon: kinds[0], File: kinds[1], Shmem: kinds[2]}, nil
}

// Header is the CSV's columns, after a comment of the process's start.
const Header = "unix_ns,vm_rss,vm_hwm,utime_ns,stime_ns,memory_current,cpu_usage_usec,anon,shmem,file"

// CSV is the row as a line of Header's columns.
func (r Row) CSV() string {
	return fmt.Sprintf("%d,%d,%d,%d,%d,%d,%d,%d,%d,%d", r.At.UnixNano(), r.RSS, r.HWM, r.UserNS, r.SystemNS, r.MemoryCurrent,
		r.ContainerCPUMicro, r.Anon, r.Shmem, r.File)
}

// Run writes to w a comment of the process and its start, Header, then a
// row every every until ctx ends, then a comment of the final values and
// of the rows' largest anon, shmem and file. A read that fails ends it
// with its error, after the final values when they can be read.
func (s *Sampler) Run(ctx context.Context, w io.Writer, every time.Duration, now func() time.Time) error {
	if _, err := fmt.Fprintf(w, "# pid=%d clk_tck=%d started_unix_ns=%d\n%s\n", s.pid, s.tick, s.started.UnixNano(), Header); err != nil {
		return err
	}
	tick := time.NewTicker(every)
	defer tick.Stop()
	var most Final
	final := func() error {
		f, err := s.Final()
		if err != nil {
			return err
		}
		f.MaxAnon, f.MaxShmem, f.MaxFile = most.MaxAnon, most.MaxShmem, most.MaxFile
		_, err = fmt.Fprintf(w, "# final memory_peak=%d anon=%d file=%d shmem=%d max_anon=%d max_shmem=%d max_file=%d\n", f.Peak, f.Anon,
			f.File, f.Shmem, f.MaxAnon, f.MaxShmem, f.MaxFile)
		return err
	}
	for {
		r, err := s.Sample(now())
		if err != nil {
			_ = final()
			return err
		}
		most.MaxAnon, most.MaxShmem, most.MaxFile = max(most.MaxAnon, r.Anon), max(most.MaxShmem, r.Shmem), max(most.MaxFile, r.File)
		if _, err := fmt.Fprintln(w, r.CSV()); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return final()
		}
		select {
		case <-ctx.Done():
			return final()
		case <-tick.C:
		}
	}
}
