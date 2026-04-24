package githubapp

import (
	"bufio"
	"context"
	"fmt"
	"io"
)

// AppInstallChecker is the minimal detection surface the install flow
// needs. The concrete *Checker in this package satisfies it; tests inject
// a fake.
type AppInstallChecker interface {
	CheckRepoInstallation(ctx context.Context, owner, repo string) (bool, int64, error)
	CheckOrgInstallation(ctx context.Context, org string) (bool, int64, error)
}

// Target describes the repo-or-org install target for EnsureInstalled.
// Owner always holds the GitHub login; Repo is populated only when Type
// is TargetRepo.
type Target struct {
	Type  TargetType
	Owner string
	Repo  string
}

// Name returns a human-readable identifier — "owner" for org targets,
// "owner/repo" for repo targets. Used in log lines and prompts.
func (t Target) Name() string {
	if t.Type == TargetOrg {
		return t.Owner
	}
	return t.Owner + "/" + t.Repo
}

// Flow is the injectable bundle of behaviour the install-guidance helper
// drives. Every callable is substitutable so tests can run the full
// decision tree without touching the network, the terminal, or stdin.
type Flow struct {
	// Checker performs the detection lookup against GitHub.
	Checker AppInstallChecker

	// Slug is the app slug used when building the install URL.
	Slug string

	// OpenURL opens a URL in the user's browser. When nil, the interactive
	// path falls back to printing the URL and continues.
	OpenURL func(url string) error

	// Confirm gates the interactive install prompt. When nil, the flow
	// treats the user as having declined (it never assumes consent).
	Confirm func(title, description string) (bool, error)

	// IsCI reports whether the process is headless (CI, non-TTY). When
	// nil, the flow assumes interactive — callers always wire a real
	// detector in production.
	IsCI func() bool

	// IsInteractive reports whether the given writer is a TTY. When nil,
	// TTY detection is skipped and only IsCI drives headless routing.
	IsInteractive func(w io.Writer) bool

	// WaitEnter blocks until the user presses Enter, used after the
	// browser-open step in the interactive flow. When nil, the flow
	// skips the wait and re-checks immediately.
	WaitEnter func(r io.Reader) error
}

// Result reports which branch of the flow executed. Callers may use it
// for logging or test assertions; no branch is treated as an error — the
// command always continues (headless and declined both succeed).
type Result int

const (
	// ResultAlreadyInstalled — no-op: the App was already installed.
	ResultAlreadyInstalled Result = iota

	// ResultInstalledInteractive — the user accepted the prompt, the
	// browser was opened, and the re-check confirmed the install.
	ResultInstalledInteractive

	// ResultPendingInteractive — the user accepted the prompt, but the
	// re-check still shows the App as not installed. A warning has been
	// logged to the writer.
	ResultPendingInteractive

	// ResultDeclinedInteractive — the user declined the prompt. The URL
	// has been logged for them to use manually.
	ResultDeclinedInteractive

	// ResultHeadless — headless environment (CI or non-TTY). The URL
	// has been logged for the operator to use manually.
	ResultHeadless
)

// EnsureInstalled runs the four-path agentic-App install flow against a
// target. It never fails the command on the install-flow itself — only
// irrecoverable detection errors return a non-nil error. User-decline,
// headless mode, and "still not installed after browser open" all return
// nil so the surrounding wizard can continue.
//
// Output is written to w using terse, pipeline-friendly lines. stdin is
// only consumed on the interactive-accept path, and only when
// Flow.WaitEnter is set.
func EnsureInstalled(ctx context.Context, w io.Writer, stdin io.Reader, flow *Flow, target Target) (Result, error) {
	if flow == nil || flow.Checker == nil {
		return 0, fmt.Errorf("install flow is not configured")
	}
	if target.Owner == "" || (target.Type == TargetRepo && target.Repo == "") {
		return 0, fmt.Errorf("install target is incomplete: %+v", target)
	}

	installed, err := flow.check(ctx, target)
	if err != nil {
		return 0, fmt.Errorf("checking App installation: %w", err)
	}
	if installed {
		fmt.Fprintf(w, "  GitHub App already installed on %s — skipping install step\n", target.Name())
		return ResultAlreadyInstalled, nil
	}

	url := InstallURL(flow.Slug, target.Type, target.Name())

	// Headless path — the commonest CI case. No prompt, no browser, just
	// the URL and a continue.
	if flow.isHeadless(w) {
		fmt.Fprintf(w, "  Install the agentic GitHub App at %s before running the pipeline.\n", url)
		return ResultHeadless, nil
	}

	// Interactive prompt. If Confirm is nil we treat that as a decline —
	// we never assume consent to open a browser.
	accepted := false
	if flow.Confirm != nil {
		ok, cerr := flow.Confirm(
			"Install agentic GitHub App?",
			fmt.Sprintf("The agentic GitHub App is not installed on %s. Open the install page in your browser?", target.Name()),
		)
		if cerr == nil {
			accepted = ok
		}
	}
	if !accepted {
		fmt.Fprintf(w, "  Install the agentic GitHub App at %s when ready.\n", url)
		return ResultDeclinedInteractive, nil
	}

	// User accepted — open the browser. A failure to launch is logged
	// but does not abort the wizard; the URL is still shown.
	if flow.OpenURL != nil {
		if oerr := flow.OpenURL(url); oerr != nil {
			fmt.Fprintf(w, "  Could not open browser automatically (%v) — install manually at %s\n", oerr, url)
		}
	} else {
		fmt.Fprintf(w, "  Open the install page at %s\n", url)
	}

	// Wait for the user to click through. The prompter is optional — when
	// unset (e.g. in tests) we skip straight to the re-check.
	if flow.WaitEnter != nil {
		fmt.Fprintln(w, "  Waiting for you to complete installation — press Enter when done.")
		_ = flow.WaitEnter(stdin)
	}

	installed, err = flow.check(ctx, target)
	if err == nil && installed {
		fmt.Fprintf(w, "  GitHub App confirmed on %s.\n", target.Name())
		return ResultInstalledInteractive, nil
	}
	fmt.Fprintf(w, "  Still not detected on %s — finish install at %s and re-run the command if needed.\n", target.Name(), url)
	return ResultPendingInteractive, nil
}

// isHeadless is true when either IsCI reports CI or IsInteractive reports
// non-TTY for the writer. Either signal is enough — callers that want to
// force the interactive path in tests can leave both callbacks nil, which
// returns false and routes the flow to the prompt.
func (f *Flow) isHeadless(w io.Writer) bool {
	if f.IsCI != nil && f.IsCI() {
		return true
	}
	if f.IsInteractive != nil && !f.IsInteractive(w) {
		return true
	}
	return false
}

// check dispatches to the correct REST endpoint for the target's type.
func (f *Flow) check(ctx context.Context, t Target) (bool, error) {
	switch t.Type {
	case TargetOrg:
		installed, _, err := f.Checker.CheckOrgInstallation(ctx, t.Owner)
		return installed, err
	default:
		installed, _, err := f.Checker.CheckRepoInstallation(ctx, t.Owner, t.Repo)
		return installed, err
	}
}

// WaitEnterFromReader is the default WaitEnter implementation — it reads
// a single line from r, discarding the content. Production wiring passes
// os.Stdin; tests may pass a bytes.Reader with a newline.
func WaitEnterFromReader(r io.Reader) error {
	if r == nil {
		return nil
	}
	scanner := bufio.NewScanner(r)
	scanner.Scan()
	return scanner.Err()
}
