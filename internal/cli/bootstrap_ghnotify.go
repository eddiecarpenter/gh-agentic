package cli

import (
	"fmt"
	"io"
	"path/filepath"

	"github.com/eddiecarpenter/gh-agentic/internal/bootstrap"
	"github.com/eddiecarpenter/gh-agentic/internal/ui"
)

// PromptGhNotify offers to install the gh-notify LaunchAgent after a
// successful bootstrap. It is a no-op on non-darwin systems. The confirm
// parameter is injected so tests can stub the interactive prompt.
// The function never returns a fatal error by design — install failure
// prints a warning but does not fail the bootstrap.
func PromptGhNotify(w io.Writer, goos string, clonePath string, run bootstrap.RunCommandFunc, confirm bootstrap.ConfirmFunc) error {
	if goos != "darwin" {
		return nil
	}

	ok, err := confirm("Install GitHub desktop notifications (gh-notify LaunchAgent)?")
	if err != nil {
		return err
	}
	if !ok {
		return nil
	}

	scriptPath := filepath.Join(clonePath, "base", "scripts", "install-gh-notify.sh")
	_, runErr := run("bash", scriptPath)
	if runErr != nil {
		fmt.Fprintln(w, "  "+ui.RenderWarning(fmt.Sprintf("gh-notify install failed: %v", runErr)))
		return nil
	}

	fmt.Fprintln(w, "  "+ui.RenderOK("gh-notify installed and running"))
	return nil
}
