package proc

import (
	"fmt"

	sys "golang.org/x/sys/unix"
)

type WaitStatus sys.WaitStatus

// OSSpecificDetails hold Linux specific
// process details.
type OSSpecificDetails struct {
	registers sys.PtraceRegs
}

func (t *Thread) halt() (err error) {
	err = sys.Tgkill(t.dbp.Pid, t.ID, sys.SIGSTOP)
	if err != nil {
		err = fmt.Errorf("halt err %s on thread %d", err, t.ID)
		return
	}
	_, _, err = t.dbp.wait(t.ID, 0)
	if err != nil {
		err = fmt.Errorf("wait err %s on thread %d", err, t.ID)
		return
	}
	return
}

func (t *Thread) stopped() bool {
	state := status(t.ID, t.dbp.os.comm)
	return state == StatusTraceStop || state == StatusTraceStopT
}

func (t *Thread) resume() error {
	return t.resumeWithSig(0)
}

func (t *Thread) resumeWithSig(sig int) (err error) {
	t.running = true
	err = PtraceCont(t.ID, sig)
	return
}

func (t *Thread) singleStep() (err error) {
	for {
		err = PtraceSingleStep(t.ID)
		if err != nil {
			return err
		}
		wpid, status, err := t.dbp.wait(t.ID, 0)
		if err != nil {
			return err
		}
		if (status == nil || status.Exited()) && wpid == t.dbp.Pid {
			t.dbp.postExit()
			rs := 0
			if status != nil {
				rs = status.ExitStatus()
			}
			return ProcessExitedError{Pid: t.dbp.Pid, Status: rs}
		}
		if wpid == t.ID && status.StopSignal() == sys.SIGTRAP {
			return nil
		}
	}
}

func (t *Thread) blocked() bool {
	pc, _ := t.PC()
	fn := t.dbp.dwarf.PCToFunc(pc)
	if fn != nil && ((fn.Name == "runtime.futex") || (fn.Name == "runtime.usleep") || (fn.Name == "runtime.clone")) {
		return true
	}
	return false
}

func (t *Thread) saveRegisters() (Registers, error) {
	err := PtraceGetRegs(t.ID, &t.os.registers)
	if err != nil {
		return nil, fmt.Errorf("could not save register contents: %v", err)
	}
	return &Regs{&t.os.registers}, nil
}

func (t *Thread) restoreRegisters() error {
	return PtraceSetRegs(t.ID, &t.os.registers)
}
