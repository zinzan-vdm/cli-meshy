// Command harness runs the four assertions of the live mesh. It runs in the
// network namespace of node-b, so every probe leaves from that node. The
// control mode runs in node-a and sends SIGHUP to the Nebula process that it
// started.
package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "assert":
		return runAssert(args[1:])
	case "control":
		return runControl(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "harness: unknown command %q\n", args[0])
		usage()
		return 2
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `Usage:
  harness assert [flags]    run the four assertions from node-b
  harness control [flags]   run Nebula in node-a and reload it on POST /reload
`)
}

func runAssert(args []string) int {
	flags := flag.NewFlagSet("assert", flag.ContinueOnError)
	peer := flags.String("peer", "10.42.0.2", "overlay address of node-a")
	from := flags.String("from", "node-b", "node that the probes leave")
	allowed := flags.String("allowed", "8080", "port that the mesh allows")
	denied := flags.String("denied", "9090", "port that the mesh denies")
	control := flags.String("control", "http://node-a:9000/reload", "reload endpoint of node-a")
	ready := flags.Duration("wait", 30*time.Second, "time for the mesh to come up")
	reloadWait := flags.Duration("reload-wait", 20*time.Second, "time for the reload to take effect")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	fmt.Printf("e2e: waiting up to %s for %s to answer\n", *ready, *peer)
	err := wait(*ready, 500*time.Millisecond, func() error {
		return ExpectPing(*from, *peer, pingOnce)
	})
	if err != nil {
		return fail("assertion 1: %v", err)
	}
	fmt.Printf("ok 1 ping %s from %s\n", *peer, *from)

	if err := ExpectOpen(*from, *peer+":"+*allowed, 5*time.Second, dialTCP); err != nil {
		return fail("assertion 2: %v", err)
	}
	fmt.Printf("ok 2 %s:%s from %s\n", *peer, *allowed, *from)

	if err := ExpectTimeout(*from, *peer+":"+*denied, 3*time.Second, dialTCP); err != nil {
		return fail("assertion 3: %v", err)
	}
	fmt.Printf("ok 3 %s:%s from %s times out\n", *peer, *denied, *from)

	if err := wait(10*time.Second, 500*time.Millisecond, func() error {
		return requestReload(*control)
	}); err != nil {
		return fail("assertion 4: node-a did not reload: %v", err)
	}
	err = wait(*reloadWait, 500*time.Millisecond, func() error {
		return ExpectOpen("node-b", *peer+":"+*denied, 2*time.Second, dialTCP)
	})
	if err != nil {
		return fail("assertion 4: node-b cannot reach %s after node-a reloaded: %v", *denied, err)
	}
	fmt.Printf("ok 4 node-b reaches %s after node-a reloaded\n", *denied)

	fmt.Println("e2e: 4 assertions passed")
	return 0
}

func runControl(args []string) int {
	flags := flag.NewFlagSet("control", flag.ContinueOnError)
	config := flags.String("config", "/etc/nebula/meshy-lab/config.yml", "config that Nebula runs")
	allow := flags.String("allow", "/etc/nebula/meshy-lab/config-allow.yml", "config with the extra rule")
	listen := flags.String("listen", ":9000", "address of the reload endpoint")
	ports := flags.String("ports", "8080,9090", "ports that node-a answers on")
	nebulaPath := flags.String("nebula", "nebula", "Nebula binary")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	// The ports answer before the mesh is up, so a probe that reaches them over
	// the tunnel finds a listener and not a refusal. The firewall decides which
	// of the two a peer reaches.
	if err := servePorts(*ports); err != nil {
		fmt.Fprintf(os.Stderr, "harness: %v\n", err)
		return 1
	}

	child := exec.Command(*nebulaPath, "-config", *config)
	child.Stdout = os.Stdout
	child.Stderr = os.Stderr
	if err := child.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "harness: start %s: %v\n", *nebulaPath, err)
		return 1
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/reload", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "use POST", http.StatusMethodNotAllowed)
			return
		}
		if err := installAndHup(*allow, *config, child); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fmt.Fprintln(w, "reloaded")
	})

	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		fmt.Fprintf(os.Stderr, "harness: listen on %s: %v\n", *listen, err)
		child.Process.Kill()
		return 1
	}
	go http.Serve(listener, mux)

	if err := child.Wait(); err != nil {
		fmt.Fprintf(os.Stderr, "harness: nebula stopped: %v\n", err)
		return 1
	}
	return 0
}

// servePorts accepts a connection on each port and closes it at once. A rule
// decides whether a peer may open the port, so the listener has to exist on
// both sides of that decision. A refusal would prove the wrong thing.
func servePorts(ports string) error {
	for _, port := range strings.Split(ports, ",") {
		port = strings.TrimSpace(port)
		if port == "" {
			continue
		}
		listener, err := net.Listen("tcp", ":"+port)
		if err != nil {
			return fmt.Errorf("listen on :%s: %w", port, err)
		}
		go func(l net.Listener) {
			for {
				conn, err := l.Accept()
				if err != nil {
					return
				}
				conn.Close()
			}
		}(listener)
	}
	return nil
}

// installAndHup puts the config with the extra rule in place and signals the
// Nebula process. Nebula reloads the file on SIGHUP and keeps the tunnel up, so
// the fourth assertion proves a reload rather than a restart.
func installAndHup(allow, config string, child *exec.Cmd) error {
	body, err := os.ReadFile(allow)
	if err != nil {
		return err
	}
	temp := config + ".tmp"
	if err := os.WriteFile(temp, body, 0o600); err != nil {
		return err
	}
	if err := os.Rename(temp, config); err != nil {
		return err
	}
	return child.Process.Signal(syscall.SIGHUP)
}

func requestReload(control string) error {
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Post(control, "text/plain", nil)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 256))
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s: %s", control, response.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func dialTCP(addr string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout("tcp", addr, timeout)
}

func pingOnce(addr string) error {
	out, err := exec.Command("ping", "-n", "-c", "1", "-W", "2", addr).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func fail(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "e2e: "+format+"\n", args...)
	return 1
}
