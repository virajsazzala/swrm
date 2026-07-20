package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/virajsazzala/swrm/internal/api"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "start":
		runStartCmd(os.Args[2:])
	case "status":
		runStatusCmd(os.Args[2:])
	case "stop":
		runStopCmd(os.Args[2:])
	case "list":
		runListCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func runStartCmd(args []string) {
	fs := flag.NewFlagSet("start", flag.ExitOnError)
	outputDir := fs.String("output-dir", ".", "directory to write downloaded files to")
	socketPath := fs.String("socket", defaultSocketPath(), "unix socket path for the daemon's status/control api")
	logLevel := fs.String("log-level", "info", "daemon log verbosity: debug, info, warn, error")
	logFormat := fs.String("log-format", "text", "daemon log format: text, json")
	background := fs.Bool("d", false, "start the daemon and return immediately, without watching progress")
	fs.Parse(reorderArgs(args, map[string]bool{"d": true}))

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "usage: swrm start <torrent-file> [-output-dir dir] [-socket path] [-d]")
		os.Exit(2)
	}

	absSocketPath, err := filepath.Abs(*socketPath)
	if err != nil {
		fatalf("resolve socket path: %v", err)
	}

	absTorrentPath, err := filepath.Abs(fs.Arg(0))
	if err != nil {
		fatalf("resolve torrent path: %v", err)
	}
	if info, err := os.Stat(absTorrentPath); err != nil {
		fatalf("torrent file %q: %v", absTorrentPath, err)
	} else if info.IsDir() {
		fatalf("torrent file %q is a directory", absTorrentPath)
	}

	absOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		fatalf("resolve output dir: %v", err)
	}

	client := api.NewClient(absSocketPath)
	probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Second)
	probeStatus, probeErr := client.Status(probeCtx)
	probeCancel()
	if probeErr == nil {
		fatalf("a download is already running on socket %s (status: %s) — stop it first with `swrm stop -socket %s`",
			absSocketPath, probeStatus.Status, absSocketPath)
	}

	swrmdPath, err := findSwrmd()
	if err != nil {
		fatalf("%v", err)
	}

	if err := os.MkdirAll(filepath.Dir(absSocketPath), 0700); err != nil {
		fatalf("create socket directory: %v", err)
	}

	logPath := strings.TrimSuffix(absSocketPath, filepath.Ext(absSocketPath)) + ".log"
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fatalf("open daemon log file: %v", err)
	}
	defer logFile.Close()

	cmd := exec.Command(swrmdPath,
		"-torrent", absTorrentPath,
		"-output-dir", absOutputDir,
		"-socket", absSocketPath,
		"-log-level", *logLevel,
		"-log-format", *logFormat,
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		fatalf("start swrmd: %v", err)
	}
	if err := cmd.Process.Release(); err != nil {
		fatalf("detach swrmd: %v", err)
	}

	if !waitForSocket(absSocketPath, 5*time.Second) {
		fmt.Fprintf(os.Stderr, "swrmd did not come up in time — check the log at %s\n", logPath)
		os.Exit(1)
	}

	fmt.Printf("started swrmd for %q (socket: %s, log: %s)\n", filepath.Base(absTorrentPath), absSocketPath, logPath)

	if *background {
		return
	}

	watch(client)
}

func runStatusCmd(args []string) {
	fs := flag.NewFlagSet("status", flag.ExitOnError)
	socketPath := fs.String("socket", defaultSocketPath(), "unix socket path of the running swrmd daemon")
	watchFlag := fs.Bool("watch", false, "keep polling and show a live-updating progress line instead of a one-shot status")
	fs.Parse(args)

	client := api.NewClient(*socketPath)

	if *watchFlag {
		watch(client)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	status, err := client.Status(ctx)
	if err != nil {
		fatalf("%v", err)
	}
	printStatusBlock(os.Stdout, status)
}

func runStopCmd(args []string) {
	fs := flag.NewFlagSet("stop", flag.ExitOnError)
	socketPath := fs.String("socket", defaultSocketPath(), "unix socket path of the running swrmd daemon")
	fs.Parse(args)

	client := api.NewClient(*socketPath)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.Stop(ctx); err != nil {
		fatalf("%v", err)
	}
	fmt.Println("stop requested")
}

func runListCmd(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	dir := fs.String("dir", filepath.Dir(defaultSocketPath()), "directory to scan for swrmd unix sockets")
	clean := fs.Bool("clean", false, "remove stale/unreachable socket files instead of just reporting them")
	fs.Parse(args)

	entries, err := os.ReadDir(*dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Printf("no swrmd daemons found (%s does not exist)\n", *dir)
			return
		}
		fatalf("read %s: %v", *dir, err)
	}

	var found bool
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sock") {
			continue
		}
		found = true

		sockPath := filepath.Join(*dir, entry.Name())
		client := api.NewClient(sockPath)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		status, err := client.Status(ctx)
		cancel()

		if err != nil {
			if *clean {
				if rmErr := os.Remove(sockPath); rmErr != nil {
					fmt.Printf("%-40s  unreachable, failed to remove: %v\n", sockPath, rmErr)
				} else {
					fmt.Printf("%-40s  removed stale socket\n", sockPath)
				}
			} else {
				fmt.Printf("%-40s  unreachable (stale socket? rerun with -clean to remove)\n", sockPath)
			}
			continue
		}

		pct := 0.0
		if status.Total > 0 {
			pct = float64(status.Completed) / float64(status.Total) * 100
		}
		name := status.Name
		if name == "" {
			name = "(unknown)"
		}
		fmt.Printf("%-30s  %-13s  %6.2f%%  (%d/%d)  %8.0f B/s  socket: %s\n",
			name, status.Status, pct, status.Completed, status.Total, status.BytesPerSec, sockPath)
	}

	if !found {
		fmt.Printf("no swrmd daemons found in %s\n", *dir)
	}
}

func watch(client *api.Client) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	poll := func() (*api.StatusResponse, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		return client.Status(ctx)
	}

	status, err := poll()
	if err != nil {
		fatalf("%v", err)
	}
	printProgressLine(os.Stdout, status)
	if isTerminalStatus(status.Status) {
		fmt.Println()
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			fmt.Println()
			fmt.Println("stopped watching — the download continues in the background; use `swrm status` / `swrm stop` to check on or stop it")
			return
		case <-ticker.C:
			status, err := poll()
			if err != nil {
				fmt.Println()
				fmt.Printf("lost connection to the daemon: %v\n", err)
				return
			}
			printProgressLine(os.Stdout, status)
			if isTerminalStatus(status.Status) {
				fmt.Println()
				return
			}
		}
	}
}

func isTerminalStatus(status string) bool {
	switch status {
	case "completed", "error", "stopped":
		return true
	}
	return false
}

func printProgressLine(w io.Writer, status *api.StatusResponse) {
	pct := 0.0
	if status.Total > 0 {
		pct = float64(status.Completed) / float64(status.Total) * 100
	}
	eta := "unknown"
	if status.ETASeconds >= 0 {
		eta = fmt.Sprintf("%.0fs", status.ETASeconds)
	}
	fmt.Fprintf(w, "\r%-20s %-13s %6.2f%%  (%d/%d)  %8.0f B/s  eta %-8s  workers %-3d",
		status.Name, status.Status, pct, status.Completed, status.Total, status.BytesPerSec, eta, status.ActiveWorkers)
}

func printStatusBlock(w io.Writer, status *api.StatusResponse) {
	fmt.Fprintf(w, "name:           %s\n", status.Name)
	fmt.Fprintf(w, "status:         %s\n", status.Status)
	fmt.Fprintf(w, "pieces:         %d/%d (pending %d)\n", status.Completed, status.Total, status.Pending)
	fmt.Fprintf(w, "active workers: %d\n", status.ActiveWorkers)
	fmt.Fprintf(w, "rate:           %.0f B/s\n", status.BytesPerSec)
	if status.ETASeconds >= 0 {
		fmt.Fprintf(w, "eta:            %.0fs\n", status.ETASeconds)
	} else {
		fmt.Fprintf(w, "eta:            unknown\n")
	}
	if status.LastError != "" {
		fmt.Fprintf(w, "last error:     %s\n", status.LastError)
	}
}

// reorderArgs moves all flags (and their values) before any positional
// arguments, since Go's flag package stops parsing at the first non-flag
// token and never resumes — without this, "swrm start file.torrent -d"
// would silently ignore -d.
func reorderArgs(args []string, boolFlags map[string]bool) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		tok := args[i]
		if !strings.HasPrefix(tok, "-") {
			positional = append(positional, tok)
			continue
		}

		flags = append(flags, tok)
		name := strings.TrimLeft(tok, "-")
		if strings.ContainsRune(name, '=') || boolFlags[name] {
			continue
		}
		if i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func findSwrmd() (string, error) {
	if exe, err := os.Executable(); err == nil {
		sibling := filepath.Join(filepath.Dir(exe), "swrmd")
		if info, statErr := os.Stat(sibling); statErr == nil && !info.IsDir() {
			return sibling, nil
		}
	}
	if path, err := exec.LookPath("swrmd"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("could not find the swrmd binary — build it with `go build -o swrmd ./cmd/swrmd` and place it next to swrm or on your PATH")
}

func waitForSocket(path string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("unix", path, 200*time.Millisecond)
		if err == nil {
			conn.Close()
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func defaultSocketPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "./swrmd.sock"
	}
	return filepath.Join(home, ".swrm", "swrmd.sock")
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: swrm <start|status|stop|list> [flags]")
	fmt.Fprintln(os.Stderr, "  swrm start <torrent-file> [-output-dir dir] [-socket path] [-d]")
	fmt.Fprintln(os.Stderr, "  swrm status [-socket path] [-watch]")
	fmt.Fprintln(os.Stderr, "  swrm stop [-socket path]")
	fmt.Fprintln(os.Stderr, "  swrm list [-dir path]")
}
