package repository

import (
	"context"
	"fmt"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/pkg/models"
)


// Repository defines the interface for data access operations
type Repository interface {
	// Media operations
	SaveMedia(media *models.Media) error
	SaveMediaBatch(medias []*models.Media) error
	GetMedia(traktID int64) (*models.Media, error)
	FindMediaNotOnDisk() ([]*models.Media, error)
	FindMediaBatch(traktIDs []int64) ([]*models.Media, error)
	ProcessMediaBatches(batchSize int, processor func([]*models.Media) error) error
	ProcessMediaBatchesWithContext(ctx context.Context, batchSize int, processor func([]*models.Media) error) error
	StreamMedia(processor func(*models.Media) error) error
	StreamMediaWithContext(ctx context.Context, processor func(*models.Media) error) error
	UpdateMediaDownloadID(traktID, downloadID int64) error
	RemoveMedia(traktID int64) error
	FindAllMedia() ([]*models.Media, error)


	// Utility operations
	Close() error
}

// BoltRepository implements Repository using BoltDB
type BoltRepository struct {
	store *bolthold.Store
}

func NewBoltRepository(store *bolthold.Store) Repository {
	return &BoltRepository{store: store}
}

// Media operations
func (r *BoltRepository) SaveMedia(media *models.Media) error {
	if err := r.store.Upsert(media.Trakt, media); err != nil {
		return fmt.Errorf("failed to save media: %w", err)
	}
	return nil
}

func (r *BoltRepository) GetMedia(traktID int64) (*models.Media, error) {
	var media models.Media
	if err := r.store.Get(traktID, &media); err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}
	return &media, nil
}

func (r *BoltRepository) FindMediaNotOnDisk() ([]*models.Media, error) {
	var medias []*models.Media
	if err := r.store.Find(&medias, bolthold.Where("OnDisk").Eq(false)); err != nil {
		return nil, fmt.Errorf("failed to find media not on disk: %w", err)
	}
	return medias, nil
}

// SaveMediaBatch saves multiple media items in a single transaction
func (r *BoltRepository) SaveMediaBatch(medias []*models.Media) error {
	if len(medias) == 0 {
		return nil
	}

	tx, err := r.store.Bolt().Begin(true)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	// Track whether we've committed successfully
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()

	for _, media := range medias {
		if err := r.store.TxUpsert(tx, media.Trakt, media); err != nil {
			return fmt.Errorf("failed to save media in batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}
	committed = true
	return nil
}

// FindMediaBatch finds multiple media items by their Trakt IDs
func (r *BoltRepository) FindMediaBatch(traktIDs []int64) ([]*models.Media, error) {
	var medias []*models.Media

	// Convert []int64 to []interface{}
	ids := make([]interface{}, len(traktIDs))
	for i, id := range traktIDs {
		ids[i] = id
	}

	if err := r.store.Find(&medias, bolthold.Where("Trakt").In(ids...)); err != nil {
		return nil, fmt.Errorf("failed to find media batch: %w", err)
	}
	return medias, nil
}

// ProcessMediaBatches processes all media in batches to avoid loading everything into memory
func (r *BoltRepository) ProcessMediaBatches(batchSize int, processor func([]*models.Media) error) error {
	return r.ProcessMediaBatchesWithContext(context.Background(), batchSize, processor)
}

// ProcessMediaBatchesWithContext processes all media in batches with context support
func (r *BoltRepository) ProcessMediaBatchesWithContext(ctx context.Context, batchSize int, processor func([]*models.Media) error) error {
	var lastID int64 = -1

	for {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		var batch []*models.Media

		// Use cursor-based pagination for better performance
		if err := r.store.Find(&batch, bolthold.Where("Trakt").Gt(lastID).SortBy("Trakt").Limit(batchSize)); err != nil {
			return fmt.Errorf("failed to find media batch: %w", err)
		}

		if len(batch) == 0 {
			break // No more records
		}

		if err := processor(batch); err != nil {
			return fmt.Errorf("failed to process media batch: %w", err)
		}

		// Update lastID for next iteration
		lastID = batch[len(batch)-1].Trakt

		// If we got fewer records than batch size, we're done
		if len(batch) < batchSize {
			break
		}
	}

	return nil
}

// StreamMedia processes media one by one without loading all into memory
func (r *BoltRepository) StreamMedia(processor func(*models.Media) error) error {
	return r.StreamMediaWithContext(context.Background(), processor)
}

// StreamMediaWithContext processes media one by one with context support
func (r *BoltRepository) StreamMediaWithContext(ctx context.Context, processor func(*models.Media) error) error {
	return r.store.ForEach(nil, func(record interface{}) error {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		media, ok := record.(*models.Media)
		if !ok {
			return fmt.Errorf("unexpected type: %T", record)
		}
		return processor(media)
	})
}

func (r *BoltRepository) UpdateMediaDownloadID(traktID, downloadID int64) error {
	return r.store.UpdateMatching(&models.Media{},
		bolthold.Where("Trakt").Eq(traktID),
		func(record interface{}) error {
			media := record.(*models.Media)
			media.DownloadID = downloadID
			return nil
		})
}

func (r *BoltRepository) RemoveMedia(traktID int64) error {
	if err := r.store.Delete(traktID, &models.Media{}); err != nil {
		return fmt.Errorf("failed to remove media: %w", err)
	}
	return nil
}

func (r *BoltRepository) FindAllMedia() ([]*models.Media, error) {
	var medias []*models.Media
	if err := r.store.Find(&medias, nil); err != nil {
		return nil, fmt.Errorf("failed to find all media: %w", err)
	}
	return medias, nil
}


// Utility operations
func (r *BoltRepository) Close() error {
	if err := r.store.Close(); err != nil {
		return fmt.Errorf("failed to close repository: %w", err)
	}
	return nil
}
