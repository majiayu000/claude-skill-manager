package cmd

import (
	"strings"
	"testing"
)

func TestListCommandsReturnHomeError(t *testing.T) {
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	t.Chdir(t.TempDir())
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	for _, name := range []string{"list", "update"} {
		t.Run(name, func(t *testing.T) {
			rootCmd.SetArgs([]string{name})
			err := rootCmd.Execute()
			if err == nil || !strings.Contains(err.Error(), "failed to resolve home directory") {
				t.Fatalf("expected a home directory error from %s, got %v", name, err)
			}
		})
	}
}
