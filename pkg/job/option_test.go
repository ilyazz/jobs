package job

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"os/exec"
	"testing"
)

func TestUidOption(t *testing.T) {

	startCommand = func(c *exec.Cmd) error {
		return nil
	}
	appFs = afero.NewMemMapFs()

	jDir := t.TempDir()

	j, err := New("ls", []string{"/tmp", "/var"}, dir(jDir), cgroup(t.TempDir()), UID(222))
	assert.NoError(t, err)

	assert.Equal(t, 222, j.ids.UID)
}

//TODO add tests for other options
