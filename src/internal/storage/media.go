package storage

import (
	"context"
	"fmt"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/internal/domain"
)

const (
	errDuplicateKey = "This Key already exists in this bolthold for this type"
)

type mediaRepository struct {
	store *bolthold.Store
}

func NewMediaRepository(store *bolthold.Store) domain.MediaRepository {
	return &mediaRepository{store: store}
}

func (r *mediaRepository) Insert(ctx context.Context, key int64, media *domain.Media) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	err := r.store.Insert(key, media)
	if err != nil && err.Error() == errDuplicateKey {
		return domain.ErrDuplicateKey
	}
	if err != nil {
		return fmt.Errorf("inserting media: %w", err)
	}
	return nil
}

func (r *mediaRepository) Update(ctx context.Context, key int64, media *domain.Media) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := r.store.Update(key, media); err != nil {
		return fmt.Errorf("updating media: %w", err)
	}
	return nil
}

func (r *mediaRepository) Get(ctx context.Context, key int64) (*domain.Media, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var media domain.Media
	if err := r.store.Get(key, &media); err != nil {
		return nil, fmt.Errorf("getting media: %w", err)
	}
	return &media, nil
}

func (r *mediaRepository) Delete(ctx context.Context, key int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var media domain.Media
	if err := r.store.Delete(key, &media); err != nil {
		return fmt.Errorf("deleting media: %w", err)
	}
	return nil
}

func (r *mediaRepository) FindNotOnDisk(ctx context.Context) ([]domain.Media, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var medias []domain.Media
	err := r.store.Find(&medias, bolthold.Where("OnDisk").Eq(false).SortBy("TraktID"))
	if err != nil {
		return nil, fmt.Errorf("finding media not on disk: %w", err)
	}
	return medias, nil
}

func (r *mediaRepository) FindNotInList(ctx context.Context, traktIDs []int64) ([]domain.Media, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	interfaceIDs := convertToInterfaceSlice(traktIDs)
	var medias []domain.Media
	err := r.store.Find(&medias, bolthold.Where("TraktID").Not().ContainsAny(interfaceIDs...))
	if err != nil {
		return nil, fmt.Errorf("finding media not in list: %w", err)
	}
	return medias, nil
}

func (r *mediaRepository) FindWithIMDB(ctx context.Context) ([]domain.Media, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	var medias []domain.Media
	err := r.store.Find(&medias, bolthold.Where("IMDB").Ne(""))
	if err != nil {
		return nil, fmt.Errorf("finding media with IMDB: %w", err)
	}
	return medias, nil
}

func (r *mediaRepository) Close() error {
	return r.store.Close()
}

func convertToInterfaceSlice(ids []int64) []interface{} {
	result := make([]interface{}, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}
