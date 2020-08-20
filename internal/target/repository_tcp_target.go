package target

import (
	"context"
	"fmt"

	"github.com/hashicorp/boundary/internal/db"
	"github.com/hashicorp/boundary/internal/kms"
	"github.com/hashicorp/boundary/internal/oplog"
	wrapping "github.com/hashicorp/go-kms-wrapping"
)

// CreateTcpTarget inserts into the repository and returns the new Target with
// its list of host sets.  WithHostSets is currently the only supported option.
func (r *Repository) CreateTcpTarget(ctx context.Context, keyWrapper wrapping.Wrapper, target *TcpTarget, opt ...Option) (Target, []string, error) {
	opts := getOpts(opt...)
	if keyWrapper == nil {
		return nil, nil, fmt.Errorf("create tcp target: missing key wrapper: %w", db.ErrNilParameter)
	}
	if target == nil {
		return nil, nil, fmt.Errorf("create tcp target: missing target: %w", db.ErrNilParameter)
	}
	if target.TcpTarget == nil {
		return nil, nil, fmt.Errorf("create tcp target: missing target store: %w", db.ErrNilParameter)
	}
	if target.ScopeId == "" {
		return nil, nil, fmt.Errorf("create tcp target: scope id empty: %w", db.ErrInvalidParameter)
	}
	if target.Name == "" {
		return nil, nil, fmt.Errorf("create tcp target: name empty: %w", db.ErrInvalidParameter)
	}
	if target.PublicId != "" {
		return nil, nil, fmt.Errorf("create tcp target: public id not empty: %w", db.ErrInvalidParameter)
	}
	id, err := newTcpTargetId()
	if err != nil {
		return nil, nil, fmt.Errorf("create tcp target: %w", err)
	}
	newHostSets := make([]interface{}, 0, len(opts.withHostSets))
	for _, id := range opts.withHostSets {
		hostSet, err := NewTargetHostSet(id, id)
		if err != nil {
			return nil, nil, fmt.Errorf("create tcp target: unable to create in memory target host set: %w", err)
		}
		newHostSets = append(newHostSets, hostSet)
	}

	oplogWrapper, err := r.kms.GetWrapper(ctx, target.ScopeId, kms.KeyPurposeOplog)
	if err != nil {
		return nil, nil, fmt.Errorf("create tcp target: unable to get oplog wrapper: %w", err)
	}
	t := target.Clone().(*TcpTarget)
	t.PublicId = id

	metadata := t.oplog(oplog.OpType_OP_TYPE_CREATE)
	var returnedTarget interface{}
	_, err = r.writer.DoTx(
		ctx,
		db.StdRetryCnt,
		db.ExpBackoff{},
		func(_ db.Reader, w db.Writer) error {
			targetTicket, err := w.GetTicket(t)
			if err != nil {
				return fmt.Errorf("create tcp target: unable to get ticket: %w", err)
			}
			msgs := make([]*oplog.Message, 0, 2)
			var targetOplogMsg oplog.Message
			returnedTarget = t.Clone()
			if err := w.Create(ctx, returnedTarget, db.NewOplogMsg(&targetOplogMsg)); err != nil {
				return err
			}
			msgs = append(msgs, &targetOplogMsg)
			if len(newHostSets) > 0 {
				hostSetOplogMsgs := make([]*oplog.Message, 0, len(newHostSets))
				if err := w.CreateItems(ctx, newHostSets, db.NewOplogMsgs(&hostSetOplogMsgs)); err != nil {
					return fmt.Errorf("create tcp target: unable to add host sets: %w", err)
				}
				msgs = append(msgs, hostSetOplogMsgs...)
			}
			if err := w.WriteOplogEntryWith(ctx, oplogWrapper, targetTicket, metadata, msgs); err != nil {
				return fmt.Errorf("create tcp target: unable to write oplog: %w", err)
			}
			return nil
		},
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create tcp target: %w for %s target id id", err, t.PublicId)
	}
	return returnedTarget.(*TcpTarget), nil, err
}