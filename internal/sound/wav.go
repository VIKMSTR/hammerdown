package sound

import (
	"encoding/binary"
	"errors"
	"fmt"
)

// decodeWAV reads a 16-bit PCM mono WAV file and returns its samples scaled
// to [-1, 1] and its sample rate. It only understands what the bundled
// assets use; anything else is an error.
func decodeWAV(b []byte) ([]float32, int, error) {
	if len(b) < 12 || string(b[0:4]) != "RIFF" || string(b[8:12]) != "WAVE" {
		return nil, 0, errors.New("not a RIFF/WAVE file")
	}
	var (
		rate     int
		haveFmt  bool
		channels uint16
		bits     uint16
	)
	for p := 12; p+8 <= len(b); {
		id := string(b[p : p+4])
		size := int(binary.LittleEndian.Uint32(b[p+4 : p+8]))
		body := p + 8
		if size < 0 || body+size > len(b) {
			return nil, 0, fmt.Errorf("chunk %q overruns file", id)
		}
		switch id {
		case "fmt ":
			if size < 16 {
				return nil, 0, errors.New("short fmt chunk")
			}
			format := binary.LittleEndian.Uint16(b[body:])
			channels = binary.LittleEndian.Uint16(b[body+2:])
			rate = int(binary.LittleEndian.Uint32(b[body+4:]))
			bits = binary.LittleEndian.Uint16(b[body+14:])
			if format != 1 || channels != 1 || bits != 16 {
				return nil, 0, fmt.Errorf("unsupported format %d, %d channels, %d bits", format, channels, bits)
			}
			haveFmt = true
		case "data":
			if !haveFmt {
				return nil, 0, errors.New("data chunk before fmt chunk")
			}
			n := size / 2
			out := make([]float32, n)
			for i := range out {
				out[i] = float32(int16(binary.LittleEndian.Uint16(b[body+2*i:]))) / 32768
			}
			return out, rate, nil
		}
		p = body + size + size&1 // chunks are word aligned
	}
	return nil, 0, errors.New("no data chunk")
}
