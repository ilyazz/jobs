package integration

import (
	"flag"
	"fmt"
	"github.com/stretchr/testify/assert"
	"os"
	"os/exec"
	"strings"
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
	c.Stdin = os.Stdin
	c.Stderr = os.Stderr

	err := c.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to start the server", err)
		os.Exit(1)
	}
	// let server start
	time.Sleep(time.Second)
	m.Run()

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
	assert.NoError(t, err, "failed to run ps")

	id := outToJID(out)

	c = exec.Command(*client, "--config", "assets/test-client.yaml", "logs", id)
	out, err = c.CombinedOutput()
	assert.NoError(t, err, "failed to get logs")
}

func TestPsBadCA(t *testing.T) {
	c := exec.Command(*client, "--config", "assets/test-client.yaml", "run", "ps", "aux")
	out, err := c.CombinedOutput()
	assert.NoError(t, err, "failed to run ps")

	id := outToJID(out)

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"--key", "../cert/client/other-john-key.pem",
		"--cert", "../cert/client/other-john-cert.pem",
		"logs", id)
	out, err = c.CombinedOutput()
	assert.Error(t, err, "a user with the same name but different CA must not have access to job log")
}

func TestLogAccess(t *testing.T) {

	c := exec.Command(*client, "--config", "assets/test-client.yaml",
		"run", "ps", "aux")
	out, err := c.CombinedOutput()
	assert.NoError(t, err, "failed to run ps")

	id := outToJID(out)

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"logs", id)
	_, err = c.CombinedOutput()
	assert.NoError(t, err, "Author user must have access to logs")

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"--key", "assets/cert/client/george-key.pem",
		"--cert", "assets/cert/client/george-cert.pem",
		"logs", id)
	out, err = c.CombinedOutput()
	assert.NoError(t, err, "Super-user user must have access to logs")

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"--key", "assets/cert/client/ringo-key.pem",
		"--cert", "assets/cert/client/ringo-cert.pem",
		"logs", id)
	out, err = c.CombinedOutput()
	assert.NoError(t, err, "Super-read-user user must have access to logs")

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"--key", "assets/cert/client/paul-key.pem",
		"--cert", "assets/cert/client/paul-cert.pem",
		"logs", id)
	out, err = c.CombinedOutput()
	assert.Error(t, err, "Regular other user must have no access to logs")
}

// Test jobs logs are not available after 'rm'
func TestRemove(t *testing.T) {
	c := exec.Command(*client, "--config", "assets/test-client.yaml",
		"run", "ps", "aux")
	out, err := c.CombinedOutput()
	assert.NoError(t, err, "failed to run ps")

	id := outToJID(out)
	strings.TrimRight(id, "\n")

	// avoid race here. job may be ended by this point, as well as still running
	_, _ = exec.Command(*client, "--config", "assets/test-client.yaml",
		"stop", "--force", id).CombinedOutput()

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"rm", id)
	_, err = c.CombinedOutput()
	assert.NoError(t, err, "Author user must be able to remove job")

	c = exec.Command(*client, "--config", "assets/test-client.yaml",
		"rm", id)
	_, err = c.CombinedOutput()
	assert.Error(t, err, "No logs must be available after job removal")
}

// outToJID trims newlines and whitespaces
func outToJID(id []byte) string {
	return strings.Trim(string(id), "\n \t")
}
