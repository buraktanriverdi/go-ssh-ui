package record

import "testing"

func TestCommandDetectionWithBackspaceCorrection(t *testing.T) {
	r := New(nil)
	// user typed "ssh user@hostx", noticed the typo, backspaced the 'x' and
	// typed '\r' - and threw in an ESC[A (up-arrow) beforehand that must not
	// corrupt the buffer.
	r.FeedInput("ssh user@host\x1b[Ax")
	r.FeedInput("\x7f\r")

	snap := r.Snapshot()
	if len(snap.Steps) != 1 || snap.Steps[0].Kind != StepExec || snap.Steps[0].Value != "ssh user@host" {
		t.Fatalf("got steps %+v, want single exec step %q", snap.Steps, "ssh user@host")
	}
}

func TestNonSSHLineDiscardedWhileWaitingForCommand(t *testing.T) {
	r := New(nil)
	r.FeedInput("cd ~/work\r")
	r.FeedInput("ls -la\r")
	if snap := r.Snapshot(); len(snap.Steps) != 0 {
		t.Fatalf("expected no steps captured yet, got %+v", snap.Steps)
	}

	r.FeedInput("ssh prod.example.com\r")
	snap := r.Snapshot()
	if len(snap.Steps) != 1 || snap.Steps[0].Value != "ssh prod.example.com" {
		t.Fatalf("got steps %+v", snap.Steps)
	}
}

func TestOutputIgnoredBeforeCommandDetected(t *testing.T) {
	r := New(nil)
	// A stray "Password:"-looking line before any ssh command was typed
	// (e.g. output from an unrelated command) must never start a capture.
	r.FeedOutput("cat notes.txt\r\nPassword: not-actually-a-prompt\r\n")
	r.FeedInput("hunter2\r")

	// Nothing was ever recognized as the starting ssh command, so "hunter2"
	// itself is still sitting in waitingCommand and gets discarded too.
	if snap := r.Snapshot(); len(snap.Steps) != 0 {
		t.Fatalf("expected no steps captured, got %+v", snap.Steps)
	}
}

func TestFullTwoStepCapture(t *testing.T) {
	var events []Event
	r := New(func(e Event) { events = append(events, e) })

	r.FeedInput("ssh -p 2222 deploy@prod.example.com\r")

	// zsh-style redraw noise before the real prompt line, including an ANSI
	// CSI sequence that must be stripped before matching.
	r.FeedOutput("\x1b[2K\x1b[1Gdeploy@prod.example.com's password: ")
	r.FeedInput("s3cr3t\r")

	r.FeedOutput("Last login: Mon Jan 1\r\n[sudo] password for deploy: ")
	r.FeedInput("sudopass\r")

	snap := r.Snapshot()
	if len(snap.Steps) != 3 {
		t.Fatalf("got %d steps, want 3: %+v", len(snap.Steps), snap.Steps)
	}
	if snap.Steps[0].Kind != StepExec || snap.Steps[0].Value != "ssh -p 2222 deploy@prod.example.com" {
		t.Fatalf("step 0: %+v", snap.Steps[0])
	}
	if snap.Steps[1].Kind != StepSendPass || snap.Steps[1].Secret != "s3cr3t" {
		t.Fatalf("step 1: %+v", snap.Steps[1])
	}
	if snap.Steps[2].Kind != StepSendPass || snap.Steps[2].Secret != "sudopass" || snap.Steps[2].Label != "[sudo] password for deploy:" {
		t.Fatalf("step 2: %+v", snap.Steps[2])
	}

	wantKinds := []EventKind{EventPromptDetected, EventCaptured, EventPromptDetected, EventCaptured}
	if len(events) != len(wantKinds) {
		t.Fatalf("got %d events, want %d: %+v", len(events), len(wantKinds), events)
	}
	for i, k := range wantKinds {
		if events[i].Kind != k {
			t.Errorf("event %d: got kind %q, want %q", i, events[i].Kind, k)
		}
	}
}

// TestMultiHopWithTrailingCommand is the scenario reported after the first
// version of this feature shipped: ssh into host1, answer its password
// prompt, then from *inside* host1's shell ssh again into host2, answer its
// prompt too, then run a plain command - all of it should end up in the
// replay script, in order, not just the first hop.
func TestMultiHopWithTrailingCommand(t *testing.T) {
	r := New(nil)

	r.FeedInput("ssh host1\r")
	r.FeedOutput("host1's password: ")
	r.FeedInput("pw1\r")

	r.FeedInput("ssh host2\r")
	r.FeedOutput("host2's password: ")
	r.FeedInput("pw2\r")

	r.FeedInput("sudo systemctl restart app\r")

	snap := r.Snapshot()
	want := []Step{
		{Kind: StepExec, Value: "ssh host1"},
		{Kind: StepSendPass, Label: "host1's password:", Secret: "pw1"},
		{Kind: StepExec, Value: "ssh host2"},
		{Kind: StepSendPass, Label: "host2's password:", Secret: "pw2"},
		{Kind: StepExec, Value: "sudo systemctl restart app"},
	}
	if len(snap.Steps) != len(want) {
		t.Fatalf("got %d steps, want %d: %+v", len(snap.Steps), len(want), snap.Steps)
	}
	for i, w := range want {
		if snap.Steps[i] != w {
			t.Errorf("step %d: got %+v, want %+v", i, snap.Steps[i], w)
		}
	}
}

func TestMaxStepsCap(t *testing.T) {
	r := New(nil)
	r.FeedInput("ssh host\r")
	for i := 0; i < maxSteps+10; i++ {
		r.FeedInput("echo hi\r")
	}
	if snap := r.Snapshot(); len(snap.Steps) != maxSteps {
		t.Fatalf("got %d steps, want capped at %d", len(snap.Steps), maxSteps)
	}
}
