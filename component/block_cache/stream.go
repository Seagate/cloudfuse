/*
   Licensed under the MIT License <http://opensource.org/licenses/MIT>.

   Copyright © 2023-2025 Seagate Technology LLC and/or its Affiliates
   Copyright © 2020-2025 Microsoft Corporation. All rights reserved.

   Permission is hereby granted, free of charge, to any person obtaining a copy
   of this software and associated documentation files (the "Software"), to deal
   in the Software without restriction, including without limitation the rights
   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
   copies of the Software, and to permit persons to whom the Software is
   furnished to do so, subject to the following conditions:

   The above copyright notice and this permission notice shall be included in all
   copies or substantial portions of the Software.

   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
   SOFTWARE
*/

package block_cache

import (
	"errors"
	"fmt"

	"github.com/Seagate/cloudfuse/common/config"
	"github.com/Seagate/cloudfuse/common/log"
	"github.com/Seagate/cloudfuse/internal"

	"github.com/pbnjay/memory"
)

type Stream struct {
	internal.BaseComponent
	BlockSize      int64
	BufferSize     uint64 // maximum number of blocks allowed to be stored for a file
	CachedObjLimit int32
}

type StreamOptions struct {
	BlockSize      uint64 `config:"block-size-mb"  yaml:"block-size-mb,omitempty"`
	BufferSize     uint64 `config:"buffer-size-mb" yaml:"buffer-size-mb,omitempty"`
	CachedObjLimit uint64 `config:"max-buffers"    yaml:"max-buffers,omitempty"`
	FileCaching    bool   `config:"file-caching"   yaml:"file-caching,omitempty"`
	readOnly       bool   `config:"read-only"      yaml:"-"`
}

const (
	compStream = "stream"
	mb         = 1024 * 1024
)

var _ internal.Component = &Stream{}

func (st *Stream) Name() string {
	return compStream
}

func (st *Stream) Configure(_ bool) error {
	log.Trace("Stream::Configure : %s", st.Name())
	conf := StreamOptions{}

	err := config.UnmarshalKey(compStream, &conf)
	if err != nil {
		log.Err("Stream::Configure : config error [invalid config attributes]")
		return fmt.Errorf("config error in %s [%s]", st.Name(), err.Error())
	}

	err = config.UnmarshalKey("read-only", &conf.readOnly)
	if err != nil {
		log.Err("Stream::Configure : config error [unable to obtain read-only]")
		return fmt.Errorf("config error in %s [%s]", st.Name(), err.Error())
	}

	if uint64((conf.BufferSize*conf.CachedObjLimit)*mb) > memory.FreeMemory() {
		log.Err(
			"Stream::Configure : config error, not enough free memory for provided configuration",
		)
		return errors.New("not enough free memory for provided stream configuration")
	}

	log.Info(
		"Stream to Block Cache::Configure : Buffer size %v, Block size %v, Handle limit %v, FileCaching %v, Read-only %v, StreamCacheMb %v, MaxBlocksPerFile %v",
		conf.BufferSize,
		conf.BlockSize,
		conf.CachedObjLimit,
		conf.FileCaching,
		conf.readOnly,
	)

	if conf.BlockSize > 0 {
		config.Set(compName+".block-size-mb", fmt.Sprint(conf.BlockSize))
	}
	if conf.BufferSize*conf.CachedObjLimit > 0 {
		config.Set(compName+".mem-size-mb", fmt.Sprint(conf.BufferSize*conf.CachedObjLimit))
	}
	return nil
}
