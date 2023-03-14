//go:build windows

package blobfuse_stats

import (
	"lyvecloudfuse/common/log"
	"os"
)

func createPipe(pipe string) error {
	_, err := os.Stat(pipe)
	if os.IsNotExist(err) {
		// TODO: Replace mkfifo with windows options for named pipes
		// err = syscall.Mkfifo(pipe, 0666)
		// if err != nil {
		// 	log.Err("StatsReader::createPipe : unable to create pipe [%v]", err)
		// 	return err
		// }
	} else if err != nil {
		log.Err("StatsReader::createPipe : [%v]", err)
		return err
	}
	return nil
}
