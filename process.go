package gophpfpm

import (
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/go-ini/ini"
)

// Process describes a minimalistic php-fpm config
// that runs only 1 pool
type Process struct {

	// path to php-fpm executable
	Exec string

	// path to the config file
	ConfigFile string

	// The address on which to accept FastCGI requests.
	// Valid syntaxes are: 'ip.add.re.ss:port', 'port',
	// '/path/to/unix/socket'. This option is mandatory for each pool.
	Listen string

	// path of the PID file
	PidFile string

	// path of the error log
	ErrorLog string

	// cmd stores the command of the running process
	cmd *exec.Cmd
}

// NewProcess creates a new process descriptor
func NewProcess(phpFpm string) *Process {
	return &Process{
		Exec: phpFpm,
	}
}

// SaveConfig generates config file according to the
// process attributes
func (proc *Process) SaveConfig(path string) {
	proc.ConfigFile = path
	proc.Config().SaveTo(proc.ConfigFile)
}

// Config generates an minimalistic config ini file
// in *ini.File format. You may then use SaveTo(path)
// to save it
func (proc *Process) Config() (f *ini.File) {
	f = ini.Empty()
	f.NewSection("global")
	f.Section("global").NewKey("pid", proc.PidFile)
	f.Section("global").NewKey("error_log", proc.ErrorLog)
	f.NewSection("www")
	f.Section("www").NewKey("listen", proc.Listen)
	f.Section("www").NewKey("pm", "dynamic")
	f.Section("www").NewKey("pm.max_children", "5")
	f.Section("www").NewKey("pm.start_servers", "2")
	f.Section("www").NewKey("pm.min_spare_servers", "1")
	f.Section("www").NewKey("pm.max_spare_servers", "3")
	return
}

// SetDatadir sets default config values according
// with reference to the folder prefix
//
// Equals to running these 3 statements:
//   process.PidFile  = basepath + "/phpfpm.pid"
//   process.ErrorLog = basepath + "/phpfpm.error_log"
//   process.Listen   = basepath + "/phpfpm.sock"
func (proc *Process) SetDatadir(prefix string) {
	// FIXME: add error if the prefix folder doesn't exists
	// or is not a folder
	proc.PidFile = path.Join(prefix, "phpfpm.pid")
	proc.ErrorLog = path.Join(prefix, "phpfpm.error_log")
	proc.Listen = path.Join(prefix, "phpfpm.sock")
}

// Start starts the php-fpm process
// in foreground mode instead of daemonize
func (proc *Process) Start() (stdout, stderr io.ReadCloser, err error) {
	proc.cmd = &exec.Cmd{
		Path: proc.Exec,
		Args: append([]string{proc.Exec},
			"--fpm-config", proc.ConfigFile,
			"-F",  // start foreground
			"-n",  // no php.ini file
			"-e"), // extended information
	}

	stdout, err = proc.cmd.StdoutPipe()
	if err != nil {
		return
	}

	stderr, err = proc.cmd.StderrPipe()
	if err != nil {
		return
	}

	err = proc.cmd.Start()
	if err != nil {
		return
	}

	select {
	case <-time.After(time.Second * 4):
		err = fmt.Errorf("time out")
	case <-proc.waitConn():
	}

	return
}

func (proc *Process) waitConn() <-chan net.Conn {
	chanConn := make(chan net.Conn)
	go func() {
		for {
			if conn, err := net.Dial("unix", proc.Listen); err != nil {
				time.Sleep(time.Millisecond * 2)
			} else {
				chanConn <- conn
				break
			}
		}
	}()
	return chanConn
}

// Stop stops the php-fpm process with SIGINT
// instead of killing
func (proc *Process) Stop() error {
	return proc.cmd.Process.Signal(os.Interrupt)
}

// Wait wait for the process to finish
func (proc *Process) Wait() (*os.ProcessState, error) {
	return proc.cmd.Process.Wait()
}
