//go:build windows

package stats_manager

import (
	"lyvecloudfuse/common/log"
	"os"
)

// This currently does not do anything on Windows, this will need to be replaced
// with named pipes.
func createPipe(pipe string) error {
	stMgrOpt.pollMtx.Lock()
	defer stMgrOpt.pollMtx.Unlock()

	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		// err = syscall.Mkfifo(pipe, 0666)
		// if err != nil {
		// 	log.Err("stats_manager::createPipe : unable to create pipe %v [%v]", pipe, err)
		// 	return err
		// }
	} else if err != nil {
		log.Err("stats_manager::createPipe : [%v]", err)
		return err
	}
	return nil
}
