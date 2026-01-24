package runner

import (
	"os/exec"
	"stromboli/internal/job"
	"strings"
	"syscall"
)

// Common signals that indicate crashes
var crashSignals = map[int]string{
	2:  "SIGINT",
	9:  "SIGKILL",
	11: "SIGSEGV",
	15: "SIGTERM",
	6:  "SIGABRT",
	8:  "SIGFPE",
	4:  "SIGILL",
	7:  "SIGBUS",
}

// Known error patterns that indicate task completion before crash
var taskCompletedPatterns = []string{
	"No messages returned",
	"Task completed",
	"finished successfully",
}

// IsCrash determines if an error represents a process crash vs normal failure
func IsCrash(err error) bool {
	if err == nil {
		return false
	}

	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		return false
	}

	status, ok := exitErr.Sys().(syscall.WaitStatus)
	if !ok {
		return false
	}

	// Check if killed by signal (exit code >= 128)
	if status.Signaled() {
		return true
	}

	// Exit codes 128+ typically indicate signal termination
	// 128 + signal_number (e.g., 139 = 128 + 11 = SIGSEGV)
	exitCode := status.ExitStatus()
	return exitCode > 128
}

// ExtractCrashInfo extracts crash details from an error and output
func ExtractCrashInfo(err error, output string) *job.CrashInfo {
	if err == nil {
		return nil
	}

	info := &job.CrashInfo{}

	exitErr, ok := err.(*exec.ExitError)
	if ok {
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		if ok {
			if status.Signaled() {
				signal := status.Signal()
				info.ExitCode = 128 + int(signal)
				info.Signal = signal.String()
				info.Reason = "Process killed by " + signal.String()
			} else {
				info.ExitCode = status.ExitStatus()
				// Check if it's a signal exit code (128+)
				if info.ExitCode > 128 {
					signalNum := info.ExitCode - 128
					if sigName, exists := crashSignals[signalNum]; exists {
						info.Signal = sigName
						info.Reason = "Process exited with " + sigName
					} else {
						info.Reason = "Process crashed with exit code " + string(rune(info.ExitCode))
					}
				}
			}
		}
	}

	// Capture partial output (limit to last 2000 chars for storage)
	if len(output) > 0 {
		if len(output) > 2000 {
			info.PartialOutput = "..." + output[len(output)-2000:]
		} else {
			info.PartialOutput = output
		}
	}

	// Check if task appeared to complete before crash
	info.TaskCompleted = checkTaskCompleted(output, err.Error())

	return info
}

// checkTaskCompleted checks if the task appeared to complete before the crash
func checkTaskCompleted(output, errMsg string) bool {
	combined := output + errMsg
	for _, pattern := range taskCompletedPatterns {
		if strings.Contains(combined, pattern) {
			return true
		}
	}
	return false
}
