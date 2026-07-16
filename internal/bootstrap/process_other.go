//go:build !unix && !windows

package bootstrap

func processRunning(int) bool {
	// Unknown platforms are conservative: never remove a potentially active lock.
	return true
}
