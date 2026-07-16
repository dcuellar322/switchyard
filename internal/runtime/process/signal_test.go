package process

import (
	"os"
	"os/signal"
)

func signalNotifyPlatform(channel chan<- os.Signal, signals ...os.Signal) {
	signal.Notify(channel, signals...)
}
