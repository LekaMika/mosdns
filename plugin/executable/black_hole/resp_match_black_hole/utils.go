package resp_ip_match_black_hole

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
	"github.com/IrineSistiana/mosdns/v5/pkg/utils"
	"io"
	"net/netip"
	"os"
	"strings"
)

func loadFromFile(f string) ([]string, error) {
	if len(f) > 0 {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		return loadFromReader(bytes.NewReader(b))
	}
	return nil, fmt.Errorf("file not found")
}

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

func loadExpsAndFiles(exps []string, fs []string, m *domain.MixMatcher[struct{}]) error {
	if err := loadExps(exps, m); err != nil {
		return err
	}
	if err := loadFiles(fs, m); err != nil {
		return err
	}
	return nil
}

func loadExps(exps []string, m *domain.MixMatcher[struct{}]) error {
	for i, exp := range exps {
		if err := m.Add(exp, struct{}{}); err != nil {
			return fmt.Errorf("failed to load expression #%d %s, %w", i, exp, err)
		}
	}
	return nil
}

func loadFiles(fs []string, m *domain.MixMatcher[struct{}]) error {
	for i, f := range fs {
		if err := LoadFile(f, m); err != nil {
			return fmt.Errorf("failed to load file #%d %s, %w", i, f, err)
		}
	}
	return nil
}

func LoadFile(f string, m *domain.MixMatcher[struct{}]) error {
	if len(f) > 0 {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}

		if err := domain.LoadFromTextReader[struct{}](m, bytes.NewReader(b), nil); err != nil {
			return err
		}
	}
	return nil
}
