//go:build !windows

package verifylifecycle

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	wrapperPath = "../../../docker/verify/with-postgres.sh"
	composeName = "docker-compose.verify.yml"
	fakePort    = "54321"
)

var (
	projectPattern = regexp.MustCompile(`^goformx-verify-[a-z0-9_-]+$`)
	projectFlag    = regexp.MustCompile(`(?:^| )-p (\S+)`)
	composeFlag    = regexp.MustCompile(`(?:^| )-f (\S+)`)
)

// fakeDocker records every invocation and answers Compose subcommands from
// environment knobs. It never talks to a daemon.
const fakeDocker = `#!/bin/sh
printf '%s\n' "$*" >>"$FAKE_DOCKER_LOG"
if [ "$1" = compose ] && [ "$2" = version ]; then
    if [ -n "${FAKE_DOCKER_NO_COMPOSE:-}" ]; then
        printf '%s\n' "docker: 'compose' is not a docker command." >&2
        exit 1
    fi
    printf '%s\n' 'Docker Compose version v2.0.0-fake'
    exit 0
fi
verb=
for arg in "$@"; do
    case "$arg" in
        up|down|port|ps) verb=$arg; break ;;
    esac
done
case "$verb" in
    ps) printf '%s' "${FAKE_DOCKER_PS_OUTPUT:-}"; exit 0 ;;
    up) exit "${FAKE_DOCKER_UP_STATUS:-0}" ;;
    port) printf '127.0.0.1:%s\n' "$FAKE_DOCKER_PORT"; exit 0 ;;
    down)
        if [ "${FAKE_DOCKER_DOWN_STATUS:-0}" -ne 0 ]; then
            printf '%s\n' 'fake compose down failure' >&2
        fi
        exit "${FAKE_DOCKER_DOWN_STATUS:-0}" ;;
esac
printf '%s\n' "unexpected docker invocation: $*" >&2
exit 99
`

// recorder is the verification command under test: it captures the environment
// the wrapper hands to verification and exits with RECORD_STATUS.
const recorder = `#!/bin/sh
printf 'url=%s\nproject=%s\n' "${GOFORMX_TEST_DATABASE_URL:-}" "${GOFORMX_VERIFY_PROJECT:-}" >"$RECORD_FILE"
exit "${RECORD_STATUS:-0}"
`

type harness struct {
	t          *testing.T
	dir        string
	logPath    string
	recordPath string
	recorder   string
}

type result struct {
	status int
	stderr string
	docker []string
	record map[string]string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("POSIX sh is required to exercise the lifecycle wrapper")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "bin")
	require.NoError(t, os.Mkdir(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, "docker"), []byte(fakeDocker), 0o755))
	recorderPath := filepath.Join(dir, "recorder.sh")
	require.NoError(t, os.WriteFile(recorderPath, []byte(recorder), 0o755))
	return &harness{
		t:          t,
		dir:        dir,
		logPath:    filepath.Join(dir, "docker.log"),
		recordPath: filepath.Join(dir, "record"),
		recorder:   recorderPath,
	}
}

func (h *harness) command(env map[string]string, args ...string) *exec.Cmd {
	cmd := exec.Command("sh", append([]string{wrapperPath, "--"}, args...)...)
	cmd.Env = append(os.Environ(),
		"PATH="+filepath.Join(h.dir, "bin")+string(os.PathListSeparator)+os.Getenv("PATH"),
		"FAKE_DOCKER_LOG="+h.logPath,
		"FAKE_DOCKER_PORT="+fakePort,
		"RECORD_FILE="+h.recordPath,
		"GOFORMX_VERIFY_PROJECT=",
		"GOFORMX_TEST_DATABASE_URL=",
	)
	for key, value := range env {
		cmd.Env = append(cmd.Env, key+"="+value)
	}
	cmd.WaitDelay = 15 * time.Second
	return cmd
}

func (h *harness) run(env map[string]string, args ...string) result {
	h.t.Helper()
	cmd := h.command(env, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	return result{
		status: exitStatus(h.t, err),
		stderr: stderr.String(),
		docker: h.dockerLog(),
		record: h.readRecord(),
	}
}

func (h *harness) runRecorder(env map[string]string) result {
	h.t.Helper()
	return h.run(env, "sh", h.recorder)
}

func (h *harness) dockerLog() []string {
	h.t.Helper()
	content, err := os.ReadFile(h.logPath)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(h.t, err)
	return strings.Split(strings.TrimSuffix(string(content), "\n"), "\n")
}

func (h *harness) readRecord() map[string]string {
	h.t.Helper()
	content, err := os.ReadFile(h.recordPath)
	if os.IsNotExist(err) {
		return nil
	}
	require.NoError(h.t, err)
	record := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(content)), "\n") {
		key, value, _ := strings.Cut(line, "=")
		record[key] = value
	}
	return record
}

func exitStatus(t *testing.T, err error) int {
	t.Helper()
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	return exitErr.ExitCode()
}

func projectOf(t *testing.T, line string) string {
	t.Helper()
	match := projectFlag.FindStringSubmatch(line)
	require.NotNil(t, match, "docker invocation lacks an explicit project: %q", line)
	return match[1]
}

func verb(line string) string {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return ""
	}
	for _, field := range fields[1:] {
		switch field {
		case "version", "ps", "up", "port", "down":
			return field
		}
	}
	return ""
}

func verbs(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, verb(line))
	}
	return out
}

func lineWithVerb(t *testing.T, lines []string, want string) string {
	t.Helper()
	for _, line := range lines {
		if verb(line) == want {
			return line
		}
	}
	require.Failf(t, "missing docker invocation", "no %q invocation in %q", want, lines)
	return ""
}

func TestSuccessfulRunProvisionsAndRemovesAUniquelyScopedProject(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(nil)

	require.Equal(t, 0, res.status, res.stderr)
	require.Equal(t, []string{"version", "ps", "up", "port", "down"}, verbs(res.docker))

	up := lineWithVerb(t, res.docker, "up")
	project := projectOf(t, up)
	require.Regexp(t, projectPattern, project)
	require.Contains(t, up, "up -d --wait")

	composeFile := composeFlag.FindStringSubmatch(up)
	require.NotNil(t, composeFile)
	require.Equal(t, composeName, filepath.Base(composeFile[1]))
	require.FileExists(t, composeFile[1])

	for _, line := range res.docker[1:] {
		require.Equal(t, project, projectOf(t, line), "every compose invocation is scoped to the run's project: %q", line)
		require.Contains(t, line, "-f "+composeFile[1])
	}
	require.Contains(t, lineWithVerb(t, res.docker, "ps"), "ps -a -q")
	require.Contains(t, lineWithVerb(t, res.docker, "port"), "port postgres 5432")
	require.Contains(t, lineWithVerb(t, res.docker, "down"), "down --volumes --remove-orphans")

	require.Equal(t, "postgres://goformx:testpass@127.0.0.1:"+fakePort+"/goformx?sslmode=disable", res.record["url"])
	require.Equal(t, project, res.record["project"])
	require.Contains(t, res.stderr, project)
}

func TestDistinctDefaultRunsReceiveDistinctProjects(t *testing.T) {
	t.Parallel()
	first := newHarness(t).runRecorder(nil)
	second := newHarness(t).runRecorder(nil)

	require.Equal(t, 0, first.status, first.stderr)
	require.Equal(t, 0, second.status, second.stderr)
	firstProject := projectOf(t, lineWithVerb(t, first.docker, "up"))
	secondProject := projectOf(t, lineWithVerb(t, second.docker, "up"))
	require.Regexp(t, projectPattern, firstProject)
	require.Regexp(t, projectPattern, secondProject)
	require.NotEqual(t, firstProject, secondProject)
}

func TestCleanupFailureAfterSuccessIsVisibleAndFailsTheRun(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(map[string]string{"FAKE_DOCKER_DOWN_STATUS": "7"})

	require.Equal(t, 7, res.status, res.stderr)
	require.Equal(t, "down", verb(res.docker[len(res.docker)-1]))
	project := projectOf(t, lineWithVerb(t, res.docker, "up"))
	require.Contains(t, res.stderr, "CLEANUP FAILED")
	require.Contains(t, res.stderr, "status 7")
	require.Contains(t, res.stderr, "docker compose -p "+project+" -f ")
	require.Contains(t, res.stderr, "down --volumes --remove-orphans")
}

func TestVerificationFailureIsPreservedWhenCleanupAlsoFails(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(map[string]string{"RECORD_STATUS": "3", "FAKE_DOCKER_DOWN_STATUS": "7"})

	require.Equal(t, 3, res.status, res.stderr)
	require.Equal(t, "down", verb(res.docker[len(res.docker)-1]))
	require.Contains(t, res.stderr, "CLEANUP FAILED")
	require.Contains(t, res.stderr, "status 7")
	require.Contains(t, res.stderr, "status 3")
}

func TestVerificationFailureWithCleanCleanupReturnsVerificationStatus(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(map[string]string{"RECORD_STATUS": "4"})

	require.Equal(t, 4, res.status, res.stderr)
	require.Equal(t, "down", verb(res.docker[len(res.docker)-1]))
	require.NotContains(t, res.stderr, "CLEANUP FAILED")
}

func TestSetupFailureStillRemovesThePartialProject(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(map[string]string{"FAKE_DOCKER_UP_STATUS": "5"})

	require.Equal(t, 5, res.status, res.stderr)
	require.Equal(t, []string{"version", "ps", "up", "down"}, verbs(res.docker))
	require.Nil(t, res.record, "verification must not run when PostgreSQL failed to start")
	require.Contains(t, res.stderr, "failed to start")
}

func TestMissingComposePluginFailsBeforeProvisioning(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(map[string]string{"FAKE_DOCKER_NO_COMPOSE": "1"})

	require.Equal(t, 1, res.status, res.stderr)
	require.Equal(t, []string{"version"}, verbs(res.docker))
	require.Nil(t, res.record)
	require.Contains(t, res.stderr, "Compose")
}

func TestExplicitProjectMustBeScopedToVerification(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"production", "goformx-verify-", "goformx-verify-Bad Name", "goformx-verify-a/b"} {
		h := newHarness(t)

		res := h.runRecorder(map[string]string{"GOFORMX_VERIFY_PROJECT": name})

		require.Equal(t, 2, res.status, "%q: %s", name, res.stderr)
		require.Empty(t, res.docker, "%q must be rejected before Docker is touched", name)
		require.Nil(t, res.record)
		require.Contains(t, res.stderr, "GOFORMX_VERIFY_PROJECT")
	}
}

func TestExplicitScopedProjectIsUsedVerbatim(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(map[string]string{"GOFORMX_VERIFY_PROJECT": "goformx-verify-local_1"})

	require.Equal(t, 0, res.status, res.stderr)
	for _, line := range res.docker[1:] {
		require.Equal(t, "goformx-verify-local_1", projectOf(t, line))
	}
	require.Equal(t, "goformx-verify-local_1", res.record["project"])
}

func TestExistingProjectIsNeverReused(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	res := h.runRecorder(map[string]string{
		"GOFORMX_VERIFY_PROJECT": "goformx-verify-stale",
		"FAKE_DOCKER_PS_OUTPUT":  "0123456789ab\n",
	})

	require.Equal(t, 1, res.status, res.stderr)
	require.Equal(t, []string{"version", "ps"}, verbs(res.docker))
	require.Nil(t, res.record)
	require.Contains(t, res.stderr, "goformx-verify-stale")
	require.Contains(t, res.stderr, "docker compose -p goformx-verify-stale -f ")
}

func TestUsageErrorTouchesNothing(t *testing.T) {
	t.Parallel()
	h := newHarness(t)

	cmd := exec.Command("sh", wrapperPath)
	cmd.Env = h.command(nil).Env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()

	require.Equal(t, 2, exitStatus(t, err), stderr.String())
	require.Empty(t, h.dockerLog())
	require.Contains(t, stderr.String(), "usage")
}

func TestSignalStopsVerificationAndRemovesTheProject(t *testing.T) {
	t.Parallel()
	h := newHarness(t)
	marker := filepath.Join(h.dir, "started")
	cmd := h.command(map[string]string{"MARKER": marker}, "sh", "-c", `: >"$MARKER"; exec sleep 30`)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	require.NoError(t, cmd.Start())

	require.Eventually(t, func() bool {
		_, err := os.Stat(marker)
		return err == nil
	}, 10*time.Second, 20*time.Millisecond, "verification command never started")
	require.NoError(t, cmd.Process.Signal(syscall.SIGTERM))
	err := cmd.Wait()

	require.Equal(t, 143, exitStatus(t, err), stderr.String())
	log := h.dockerLog()
	require.Equal(t, "down", verb(log[len(log)-1]))
	require.Contains(t, log[len(log)-1], "down --volumes --remove-orphans")
	require.Contains(t, stderr.String(), "signal")
}
