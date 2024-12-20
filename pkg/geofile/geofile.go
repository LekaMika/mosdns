/**
https://github.com/XTLS/Xray-core/blob/main/infra/conf/router.go
*/

package geofile

import (
	"github.com/xtls/xray-core/app/router"
	"github.com/xtls/xray-core/common/buf"
	"google.golang.org/protobuf/proto"
	"io"
	"os"
	"runtime/debug"
	"strings"
)

func LoadIP(file, code string) ([]*router.CIDR, error) {
	//bs, err := readAsset(file)
	bs, err := readAssetByCache(file)
	if err != nil {
		return nil, err
	}
	bs = find(bs, []byte(strings.ToUpper(code)))
	if bs == nil {
		return nil, err
	}
	var geoip router.GeoIP
	if err := proto.Unmarshal(bs, &geoip); err != nil {
		return nil, err
	}
	defer debug.FreeOSMemory()
	return geoip.Cidr, nil
}

func LoadSite(file, code string) ([]*router.Domain, error) {
	//bs, err := readAsset(file)
	bs, err := readAssetByCache(file)
	if err != nil {
		return nil, err
	}
	bs = find(bs, []byte(strings.ToUpper(code)))
	if bs == nil {
		return nil, err
	}
	var geosite router.GeoSite
	if err := proto.Unmarshal(bs, &geosite); err != nil {
		return nil, err
	}
	defer debug.FreeOSMemory()
	return geosite.Domain, nil
}

type fileReaderFunc func(path string) (io.ReadCloser, error)

var newFileReader fileReaderFunc = func(path string) (io.ReadCloser, error) {
	return os.Open(path)
}

func readFile(path string) ([]byte, error) {
	reader, err := newFileReader(path)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	return buf.ReadAllToBytes(reader)
}

func readAsset(file string) ([]byte, error) {
	bytes, err := readFile(file)
	if err != nil {
		return nil, err
	}
	return bytes, err
}

func find(data, code []byte) []byte {
	codeL := len(code)
	if codeL == 0 {
		return nil
	}
	for {
		dataL := len(data)
		if dataL < 2 {
			return nil
		}
		x, y := decodeVarint(data[1:])
		if x == 0 && y == 0 {
			return nil
		}
		headL, bodyL := 1+y, int(x)
		dataL -= headL
		if dataL < bodyL {
			return nil
		}
		data = data[headL:]
		if int(data[1]) == codeL {
			for i := 0; i < codeL && data[2+i] == code[i]; i++ {
				if i+1 == codeL {
					return data[:bodyL]
				}
			}
		}
		if dataL == bodyL {
			return nil
		}
		data = data[bodyL:]
	}
}

func decodeVarint(buf []byte) (x uint64, n int) {
	for shift := uint(0); shift < 64; shift += 7 {
		if n >= len(buf) {
			return 0, 0
		}
		b := uint64(buf[n])
		n++
		x |= (b & 0x7F) << shift
		if (b & 0x80) == 0 {
			return x, n
		}
	}

	// The number is too large to represent in a 64-bit value.
	return 0, 0
}
