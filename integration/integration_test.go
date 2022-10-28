package integration

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

var server = flag.String("server", "", "path to server binary")
var client = flag.String("client", "", "path to client binary")

var uid = flag.Int("uid", 0, "worker user id")
var gid = flag.Int("gid", 0, "worker group id")

func TestMain(m *testing.M) {
	flag.Parse()

	c := exec.Command("sudo", *server,
		"--config", "assets/test-server.yaml",
		fmt.Sprintf("--uid=%d", *uid),
		fmt.Sprintf("--gid=%d", *gid))

	c.Stdout = os.Stdout
	c.Stderr = os.Stderr

	err := c.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to start the server", err)
		os.Exit(1)
	}
	// let server start
	time.Sleep(time.Second)
	m.Run()

	c.Process.Signal(syscall.SIGKILL)

	fmt.Println("all done")

	os.Exit(0)
}

func TestPs(t *testing.T) {
	c := exec.Command(*client, "--config", "assets/test-client.yaml", "run", "ps", "aux")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
		return
	}

	id := string(out)

	c = exec.Command(*client, "--config", "assets/test-client.yaml", "logs", id)
	out, err = c.CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
	}

	fmt.Println(string(out))
}

func TestPsBadCA(t *testing.T) {
	c := exec.Command(*client, "--config", "assets/test-client.yaml", "run", "ps", "aux")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
		return
	}

	id := string(out)

	c = exec.Command(*client, "--config", "assets/other-test-client.yaml", "logs", id)
	out, err = c.CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
	}

	fmt.Println(string(out))
}
