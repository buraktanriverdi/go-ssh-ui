// Package record implements go-ssh-ui's "Kayıt" (recording) feature: while a
// user manually types `ssh ...` and answers its prompts in a plain local
// terminal tab - including hopping on to a second host from within the
// first, running further commands, etc. - a Recorder watches the raw
// input/output stream and turns the whole thing into an ordered, replayable
// step list, ready for the frontend's capture-then-confirm review panel
// (see PLAN.md Faz 5). It never persists or reports anything on its own -
// it just accumulates a Snapshot the caller reads once the user stops
// recording.
package record

import (
	"regexp"
	"strings"
	"sync"
)

// sshLineRegex matches a typed line that looks like the start of a real ssh
// invocation - mirrors the intent of go-ssh/ssh.go's sshCommandRegex, but
// anchored to the start of the (already trimmed) line since this is a whole
// typed command, not a substring inside a larger shell command. Only used
// to recognize where the recording *starts* (see waitingCommand below) -
// once inside a session, every typed line is captured regardless of what it
// is, since a second `ssh` hop, a `dzdo`/`sudo` escalation, or a plain
// command the user wants to end up in the replay script all look the same
// from here.
var sshLineRegex = regexp.MustCompile(`^ssh(\s|$)`)

// promptRegex matches the tail of a line that looks like a password-style
// prompt: login password, `[sudo] password for x:` / dzdo, key passphrase,
// 2FA/OTP prompts. Deliberately broad - false positives just mean a step
// gets captured that the review panel's "Yeni şifre oluştur" the user simply
// won't confirm; false negatives mean a real prompt is silently missed,
// which is the worse failure mode for a "never save silently" feature.
var promptRegex = regexp.MustCompile(`(?i)(password|passphrase|verification code|authentication code|passcode|pin).*:\s*$`)

// maxSteps bounds how many steps (exec lines + captured secrets combined) a
// single recording accumulates, as a sanity limit against an accidentally
// long-running recording rather than a real login-and-a-few-commands flow.
const maxSteps = 60

// maxTail bounds the output tail buffer kept for prompt matching - same
// order of magnitude as go-ssh/ssh.go's autoYesResponder.tail.
const maxTail = 512

type state int

const (
	waitingCommand state = iota
	watchingOutput
	waitingSecret
)

// StepKind distinguishes the two kinds of entry a recording produces.
type StepKind string

const (
	// StepExec is a line the user typed and hit Enter on, replayed verbatim
	// - the initial `ssh host1`, a second `ssh host2` hop typed once landed
	// on host1, a `dzdo -u other bash` escalation, or any other command.
	StepExec StepKind = "exec"
	// StepSendPass is a captured secret typed in response to a detected
	// password-style prompt. Label is the prompt text (also doubles as the
	// EXPECT match value on replay).
	StepSendPass StepKind = "sendpass"
)

// Step is one entry in the recorded sequence, in the order it was observed.
type Step struct {
	Kind   StepKind `json:"kind"`
	Value  string   `json:"value"`  // StepExec only: the typed line.
	Label  string   `json:"label"`  // StepSendPass only: the detected prompt text.
	Secret string   `json:"secret"` // StepSendPass only: the captured secret.
}

// Snapshot is the recording's result so far (or at Stop time). Empty until
// a typed line matched sshLineRegex - everything before that is discarded,
// so an accidental `cd`/`ls` before the real `ssh` call doesn't pollute it.
type Snapshot struct {
	Steps []Step `json:"steps"`
}

// EventKind distinguishes the two live events a Recorder fires, used to
// drive the frontend's "kayıt aktif" indicator.
type EventKind string

const (
	// EventPromptDetected fires when a password-style prompt appears in the
	// output while watching for one - Label is the detected prompt text.
	EventPromptDetected EventKind = "prompt-detected"
	// EventCaptured fires once the user's next line finishes a capture -
	// Label is the same prompt text the step was captured for.
	EventCaptured EventKind = "captured"
)

// Event is what a Recorder reports through its onEvent callback.
type Event struct {
	Kind  EventKind
	Label string
}

// Recorder is one recording session's state machine. Safe for concurrent
// FeedInput/FeedOutput/Snapshot calls - sshengine.Manager's Write and
// emitOutput can run on different goroutines.
type Recorder struct {
	mu    sync.Mutex
	state state

	lineBuf []rune // reconstructed current input line, cleared on \r/\n
	inEsc   bool   // mid CSI escape sequence on the input side, skipping it

	outTail strings.Builder // output tail, cleared on match/finalize

	pendingLabel string
	steps        []Step

	onEvent func(Event)
}

// New creates a Recorder. onEvent may be nil (no live updates wanted).
func New(onEvent func(Event)) *Recorder {
	return &Recorder{onEvent: onEvent}
}

func (r *Recorder) fire(kind EventKind, label string) {
	if r.onEvent != nil {
		r.onEvent(Event{Kind: kind, Label: label})
	}
}

// FeedInput processes a chunk of raw keystrokes written to the session
// (exactly what sshengine.Manager.Write receives from the frontend).
func (r *Recorder) FeedInput(data string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, ch := range data {
		if r.inEsc {
			// Skip until a CSI final byte (letter or '~') or, for a bare
			// two-byte escape (e.g. arrow-key variants some terminals send
			// without '['), just the next rune.
			if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') || ch == '~' {
				r.inEsc = false
			}
			continue
		}
		switch ch {
		case '\x1b': // ESC - start of a CSI/escape sequence, don't buffer it
			r.inEsc = true
		case '\r', '\n':
			r.finalizeLineLocked()
		case '\x7f', '\x08': // DEL / backspace
			if len(r.lineBuf) > 0 {
				r.lineBuf = r.lineBuf[:len(r.lineBuf)-1]
			}
		case '\x15': // Ctrl-U, kill line - common shell/readline binding
			r.lineBuf = r.lineBuf[:0]
		default:
			r.lineBuf = append(r.lineBuf, ch)
		}
	}
}

func (r *Recorder) finalizeLineLocked() {
	line := strings.TrimSpace(string(r.lineBuf))
	r.lineBuf = r.lineBuf[:0]
	if line == "" {
		return
	}
	if len(r.steps) >= maxSteps {
		return
	}

	switch r.state {
	case waitingCommand:
		if sshLineRegex.MatchString(line) {
			r.steps = append(r.steps, Step{Kind: StepExec, Value: line})
			r.state = watchingOutput
		}
		// No match: discard, stay in waitingCommand.

	case waitingSecret:
		r.steps = append(r.steps, Step{Kind: StepSendPass, Label: r.pendingLabel, Secret: line})
		label := r.pendingLabel
		r.pendingLabel = ""
		r.outTail.Reset()
		r.state = watchingOutput
		r.fire(EventCaptured, label)

	case watchingOutput:
		// A live session, no prompt currently pending - a second `ssh` hop,
		// an escalation command, or just a plain command the user wants
		// replayed. Capture it verbatim, same as go-ssh's own EXEC steps.
		r.steps = append(r.steps, Step{Kind: StepExec, Value: line})
	}
}

// FeedOutput processes a chunk of raw output from the session (exactly what
// sshengine.Manager.emitOutput sends to the frontend).
func (r *Recorder) FeedOutput(data string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.state != watchingOutput {
		// Before the first command is captured there's nothing to watch for
		// yet; while waitingSecret, a fresh prompt can't legitimately appear
		// again (and if it does before the user answers, the original
		// pendingLabel still wins - keeps the state machine simple).
		return
	}

	r.outTail.WriteString(data)
	tail := r.outTail.String()
	if len(tail) > maxTail {
		tail = tail[len(tail)-maxTail:]
		r.outTail.Reset()
		r.outTail.WriteString(tail)
	}

	line := lastLine(tail)
	if !promptRegex.MatchString(line) {
		return
	}

	r.pendingLabel = line
	r.state = waitingSecret
	r.outTail.Reset()
	r.fire(EventPromptDetected, line)
}

// lastLine strips ANSI CSI/OSC sequences from s and returns its last
// non-empty line, trimmed - the "current line" a redrawing shell prompt
// ends up showing.
func lastLine(s string) string {
	s = stripANSI(s)
	if idx := strings.LastIndexAny(s, "\r\n"); idx >= 0 {
		s = s[idx+1:]
	}
	return strings.TrimSpace(s)
}

var (
	csiRegex = regexp.MustCompile(`\x1b\[[0-9;?]*[a-zA-Z]`)
	oscRegex = regexp.MustCompile(`\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)
)

func stripANSI(s string) string {
	s = oscRegex.ReplaceAllString(s, "")
	s = csiRegex.ReplaceAllString(s, "")
	return s
}

// Snapshot returns the recording's current state - safe to call at any
// time, including after the recorder has stopped receiving feeds.
func (r *Recorder) Snapshot() Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := Snapshot{Steps: make([]Step, len(r.steps))}
	copy(out.Steps, r.steps)
	return out
}
