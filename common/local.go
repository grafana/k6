package common

import (
	"github.com/loadimpact/speedboat/actions/registry"
	"github.com/loadimpact/speedboat/master"
	"github.com/loadimpact/speedboat/worker"
)

// Runs a local, in-process Master, using all globally registered handlers.
func RunLocalMaster(inAddr, outAddr string) error {
	m, err := master.New(inAddr, outAddr)
	if err != nil {
		return err
	}
	m.Processors = registry.GlobalMasterProcessors
	go m.Run()
	return nil
}

// Runs a local, in-process Worker, using all globally registered processors.
func RunLocalWorker(inAddr, outAddr string) error {
	w, err := worker.New(inAddr, outAddr)
	if err != nil {
		return err
	}
	w.Processors = registry.GlobalProcessors
	go w.Run()
	return nil
}
