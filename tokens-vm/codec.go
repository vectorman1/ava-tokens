package tokens_vm

import (
	"github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
)

const (
	// CodecVersion is the current default codec version
	CodecVersion = 0
)

var (
	Codec codec.Manager
)

func init() {
	// Create default codec and manager
	c := linearcodec.NewDefault()
	Codec = codec.NewDefaultManager()

	// Register codec to manager with CodecVersion
	if err := Codec.RegisterCodec(CodecVersion, c); err != nil {
		panic(err)
	}
}
