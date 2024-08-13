package clean_up_ecs

import (
	"context"
	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
)

func init() {
	sequence.MustRegExecQuickSetup("clean_up_ecs", func(bq sequence.BQ, args string) (any, error) {
		return &cleanUpECS{}, nil
	})
}

var _ sequence.RecursiveExecutable = (*cleanUpECS)(nil)

type cleanUpECS struct {
}

func (e *cleanUpECS) Exec(ctx context.Context, qCtx *query_context.Context, next sequence.ChainWalker) error {
	queryOpt := qCtx.QOpt()
	for i, o := range queryOpt.Option {
		if o.Option() == dns.EDNS0SUBNET {
			queryOpt.Option = append(queryOpt.Option[:i], queryOpt.Option[i+1:]...)
		}
	}
	err := next.ExecNext(ctx, qCtx)
	if err != nil {
		return err
	}
	return nil
}
