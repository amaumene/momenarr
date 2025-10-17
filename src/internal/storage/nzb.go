package storage

import (
	"context"
	"fmt"
	"regexp"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/internal/domain"
)

type nzbRepository struct {
	store *bolthold.Store
}

func NewNZBRepository(store *bolthold.Store) domain.NZBRepository {
	return &nzbRepository{store: store}
}

func (r *nzbRepository) Insert(ctx context.Context, key string, nzb *domain.NZB) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.store.Insert(key, nzb)
	if err != nil && err.Error() == errDuplicateKey {
		return domain.ErrDuplicateKey
	}
	if err != nil {
		return fmt.Errorf("inserting nzb: %w", err)
	}
	return nil
}

func (r *nzbRepository) FindByTraktID(ctx context.Context, traktID int64, pattern string, failed bool) ([]domain.NZB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var nzbs []domain.NZB
	query := r.buildQuery(traktID, pattern, failed)
	err := r.store.Find(&nzbs, query)
	if err != nil {
		return nil, fmt.Errorf("finding nzbs: %w", err)
	}
	return nzbs, nil
}

func (r *nzbRepository) buildQuery(traktID int64, pattern string, failed bool) *bolthold.Query {
	query := bolthold.Where("TraktID").Eq(traktID).And("Failed").Eq(failed)
	if pattern != "" {
		query = query.And("Title").RegExp(regexp.MustCompile(pattern))
	}
	return query.SortBy("TotalScore").Reverse().Index("TotalScore")
}

func (r *nzbRepository) FindAll(ctx context.Context) ([]domain.NZB, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var nzbs []domain.NZB
	err := r.store.Find(&nzbs, &bolthold.Query{})
	if err != nil {
		return nil, fmt.Errorf("finding all nzbs: %w", err)
	}
	return nzbs, nil
}

func (r *nzbRepository) MarkFailed(ctx context.Context, title string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.store.UpdateMatching(&domain.NZB{}, bolthold.Where("Title").Eq(title), func(record interface{}) error {
		update, ok := record.(*domain.NZB)
		if !ok {
			return fmt.Errorf("database integrity error: invalid record type %T expected domain.NZB", record)
		}
		update.Failed = true
		return nil
	})
	if err != nil {
		return fmt.Errorf("marking nzb as failed: %w", err)
	}
	return nil
}

func (r *nzbRepository) DeleteByTraktID(ctx context.Context, traktID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.store.DeleteMatching(&domain.NZB{}, bolthold.Where("TraktID").Eq(traktID))
	if err != nil {
		return fmt.Errorf("deleting nzbs: %w", err)
	}
	return nil
}
