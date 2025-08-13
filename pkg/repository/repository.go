// Package repository provides data access layer abstractions for momenarr.
package repository

import (
	"context"
	"fmt"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/pkg/models"
	bolt "go.etcd.io/bbolt"
)

// Repository defines the interface for data access operations.
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
	GetMediaByTMDBAndSeason(tmdbID int64, season int64) ([]*models.Media, error)
	GetEpisodesBySeason(showTMDBID int64, season int64) ([]*models.Media, error)

	// Season pack operations
	SaveSeasonPack(pack *models.SeasonPack) error
	GetSeasonPack(showTMDBID int64, season int64) (*models.SeasonPack, error)
	RemoveSeasonPack(id int64) error

	// Utility operations
	Close() error
}

// BoltRepository implements Repository using BoltDB.
type BoltRepository struct {
	store *bolthold.Store
}

// NewBoltRepository creates a new BoltDB-backed repository.
func NewBoltRepository(store *bolthold.Store) Repository {
	return &BoltRepository{store: store}
}

// SaveMedia saves or updates a media item.
func (r *BoltRepository) SaveMedia(media *models.Media) error {
	if err := r.store.Upsert(media.Trakt, media); err != nil {
		return fmt.Errorf("failed to save media: %w", err)
	}
	return nil
}

// GetMedia retrieves a media item by Trakt ID.
func (r *BoltRepository) GetMedia(traktID int64) (*models.Media, error) {
	var media models.Media
	if err := r.store.Get(traktID, &media); err != nil {
		return nil, fmt.Errorf("failed to get media: %w", err)
	}
	return &media, nil
}

// FindMediaNotOnDisk returns all media items not currently on disk.
func (r *BoltRepository) FindMediaNotOnDisk() ([]*models.Media, error) {
	var medias []*models.Media
	if err := r.store.Find(&medias, bolthold.Where("OnDisk").Eq(false)); err != nil {
		return nil, fmt.Errorf("failed to find media not on disk: %w", err)
	}
	return medias, nil
}

// SaveMediaBatch saves multiple media items in a single transaction.
func (r *BoltRepository) SaveMediaBatch(medias []*models.Media) error {
	if len(medias) == 0 {
		return nil
	}

	tx, err := r.store.Bolt().Begin(true)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}

	if err := r.executeBatchSave(tx, medias); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}
	return nil
}

func (r *BoltRepository) executeBatchSave(tx *bolt.Tx, medias []*models.Media) error {
	for _, media := range medias {
		if err := r.store.TxUpsert(tx, media.Trakt, media); err != nil {
			return fmt.Errorf("failed to save media in batch: %w", err)
		}
	}
	return nil
}

// FindMediaBatch finds multiple media items by their Trakt IDs.
func (r *BoltRepository) FindMediaBatch(traktIDs []int64) ([]*models.Media, error) {
	if len(traktIDs) == 0 {
		return nil, nil
	}

	var medias []*models.Media
	ids := convertToInterfaces(traktIDs)

	if err := r.store.Find(&medias, bolthold.Where("Trakt").In(ids...)); err != nil {
		return nil, fmt.Errorf("failed to find media batch: %w", err)
	}
	return medias, nil
}

// ProcessMediaBatches processes all media in batches.
func (r *BoltRepository) ProcessMediaBatches(batchSize int, processor func([]*models.Media) error) error {
	return r.ProcessMediaBatchesWithContext(context.Background(), batchSize, processor)
}

// ProcessMediaBatchesWithContext processes media in batches with context.
func (r *BoltRepository) ProcessMediaBatchesWithContext(ctx context.Context, batchSize int, processor func([]*models.Media) error) error {
	lastID := int64(-1)

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		batch, err := r.findNextBatch(lastID, batchSize)
		if err != nil {
			return err
		}

		if len(batch) == 0 {
			break
		}

		if err := processor(batch); err != nil {
			return fmt.Errorf("failed to process media batch: %w", err)
		}

		lastID = batch[len(batch)-1].Trakt

		if len(batch) < batchSize {
			break
		}
	}

	return nil
}

// StreamMedia processes media items one by one.
func (r *BoltRepository) StreamMedia(processor func(*models.Media) error) error {
	return r.StreamMediaWithContext(context.Background(), processor)
}

// StreamMediaWithContext processes media items with context support.
func (r *BoltRepository) StreamMediaWithContext(ctx context.Context, processor func(*models.Media) error) error {
	return r.store.ForEach(nil, func(media *models.Media) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		return processor(media)
	})
}

// UpdateMediaDownloadID updates the download ID for a media item.
func (r *BoltRepository) UpdateMediaDownloadID(traktID, downloadID int64) error {
	return r.store.UpdateMatching(&models.Media{},
		bolthold.Where("Trakt").Eq(traktID),
		func(record interface{}) error {
			media := record.(*models.Media)
			media.DownloadID = downloadID
			return nil
		})
}

// RemoveMedia deletes a media item by Trakt ID.
func (r *BoltRepository) RemoveMedia(traktID int64) error {
	if err := r.store.Delete(traktID, &models.Media{}); err != nil {
		return fmt.Errorf("failed to remove media: %w", err)
	}
	return nil
}

// FindAllMedia returns all media items in the database.
func (r *BoltRepository) FindAllMedia() ([]*models.Media, error) {
	var medias []*models.Media
	if err := r.store.Find(&medias, nil); err != nil {
		return nil, fmt.Errorf("failed to find all media: %w", err)
	}
	return medias, nil
}

// GetMediaByTMDBAndSeason gets media by TMDB ID and season
func (r *BoltRepository) GetMediaByTMDBAndSeason(tmdbID int64, season int64) ([]*models.Media, error) {
	var medias []*models.Media
	query := bolthold.Where("TMDBID").Eq(tmdbID).And("Season").Eq(season)
	if err := r.store.Find(&medias, query); err != nil {
		return nil, fmt.Errorf("failed to find media by TMDB and season: %w", err)
	}
	return medias, nil
}

// GetEpisodesBySeason gets all episodes for a show's season
func (r *BoltRepository) GetEpisodesBySeason(showTMDBID int64, season int64) ([]*models.Media, error) {
	var medias []*models.Media
	query := bolthold.Where("TMDBID").Eq(showTMDBID).And("Season").Eq(season).And("Number").Gt(int64(0))
	if err := r.store.Find(&medias, query); err != nil {
		return nil, fmt.Errorf("failed to find episodes by season: %w", err)
	}
	return medias, nil
}

// SaveSeasonPack saves a season pack record
func (r *BoltRepository) SaveSeasonPack(pack *models.SeasonPack) error {
	if err := r.store.Upsert(pack.ID, pack); err != nil {
		return fmt.Errorf("failed to save season pack: %w", err)
	}
	return nil
}

// GetSeasonPack gets a season pack by show and season
func (r *BoltRepository) GetSeasonPack(showTMDBID int64, season int64) (*models.SeasonPack, error) {
	var packs []*models.SeasonPack
	query := bolthold.Where("ShowTMDBID").Eq(showTMDBID).And("Season").Eq(season)
	if err := r.store.Find(&packs, query); err != nil {
		return nil, fmt.Errorf("failed to find season pack: %w", err)
	}
	if len(packs) == 0 {
		return nil, fmt.Errorf("season pack not found")
	}
	return packs[0], nil
}

// RemoveSeasonPack removes a season pack by ID
func (r *BoltRepository) RemoveSeasonPack(id int64) error {
	if err := r.store.Delete(id, &models.SeasonPack{}); err != nil {
		return fmt.Errorf("failed to remove season pack: %w", err)
	}
	return nil
}

// Close closes the database connection.
func (r *BoltRepository) Close() error {
	if err := r.store.Close(); err != nil {
		return fmt.Errorf("failed to close repository: %w", err)
	}
	return nil
}

func convertToInterfaces(ids []int64) []interface{} {
	result := make([]interface{}, len(ids))
	for i, id := range ids {
		result[i] = id
	}
	return result
}

func (r *BoltRepository) findNextBatch(lastID int64, batchSize int) ([]*models.Media, error) {
	var batch []*models.Media

	query := bolthold.Where("Trakt").Gt(lastID).SortBy("Trakt").Limit(batchSize)
	if err := r.store.Find(&batch, query); err != nil {
		return nil, fmt.Errorf("failed to find media batch: %w", err)
	}

	return batch, nil
}
