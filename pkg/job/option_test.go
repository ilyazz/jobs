package job

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUidOption(t *testing.T) {

	appFs = afero.NewMemMapFs()

	jDir := t.TempDir()

	j, err := New("ls", []string{"/tmp", "/var"},
		cmdStart(defStart), cmdWait(defWait),
		dir(jDir),
		cgroup(t.TempDir()), UID(222))
	assert.NoError(t, err)

	assert.Equal(t, 222, j.ids.UID)
}

//TODO add tests for other options
