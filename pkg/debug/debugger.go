package debug

type Debugger struct {
	podName string
	containerName string
}

func NewDebugger() (*Debugger, error) {
	var err error

	return &Debugger{
		containerName: "step-breakpoint",
	}, err
}

func (d *Debugger) DebugMode() error {
	return nil
}