package black_hole

import (
	"bufio"
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/utils"
	"io"
	"net/netip"
	"strings"
)

// LoadFromReader loads IP list from a reader.
// It might modify the List and causes List unsorted.
func loadFromReader(reader io.Reader) ([]string, error) {
	ips := make([]string, 0)
	scanner := bufio.NewScanner(reader)

	// count how many lines we have read.
	lineCounter := 0
	for scanner.Scan() {
		lineCounter++
		s := scanner.Text()
		s = strings.TrimSpace(s)
		s = utils.RemoveComment(s, "#")
		s = utils.RemoveComment(s, " ")
		if len(s) == 0 {
			continue
		}
		addr, err := netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("invalid data at line #%d: %w", lineCounter, err)
		}
		if !addr.IsValid() {
			return nil, fmt.Errorf("%s not valid ip address", s)
		}
		ips = append(ips, s)
	}
	return ips, scanner.Err()
}
