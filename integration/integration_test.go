package integration

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
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
		"--pid=test.pid",
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

	fmt.Println("all done")

	data, err := os.ReadFile("test.pid")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to read pid file: %v", err)
	} else {
		_ = exec.Command("sudo", "kill", "-9", string(data)).Run()
	}

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

func TestPsBadCert(t *testing.T) {
	c := exec.Command(*client, "--config", "assets/test-client.yaml",
		"run", "ps", "aux")
	out, err := c.CombinedOutput()
	if err != nil {
		t.Fatal(string(out), err)
		return
	}

	id := string(out)

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"--key", "assets/cert/foo-key.pem",
		"--cert", "assets/cert/foo-cert.pem",
		"logs", id)
	out, err = c.CombinedOutput()
	if err == nil {
		t.Fatal(string(out), err)
	}

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"logs", id)
	out, err = c.CombinedOutput()
	if err == nil {
		t.Fatal(string(out), err)
	}

	fmt.Println(string(out))
}
