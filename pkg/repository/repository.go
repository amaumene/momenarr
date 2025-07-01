package repository

import (
	"fmt"
	"regexp"

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
	UpdateMediaDownloadID(traktID, downloadID int64) error
	RemoveMedia(traktID int64) error
	FindAllMedia() ([]*models.Media, error)

	// NZB operations
	SaveNZB(nzb *models.NZB) error
	SaveNZBBatch(nzbs []*models.NZB) error
	GetNZB(traktID int64) (*models.NZB, error)
	FindAllNZBsByTraktID(traktID int64) ([]*models.NZB, error)
	FindNZBsByTraktIDs(traktIDs []int64) ([]*models.NZB, error)
	RemoveNZBsByTraktID(traktID int64) error

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

func (r *BoltRepository) Store() *bolthold.Store {
	return r.store
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
	defer tx.Rollback()

	for _, media := range medias {
		if err := r.store.TxUpsert(tx, media.Trakt, media); err != nil {
			return fmt.Errorf("failed to save media in batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit batch transaction: %w", err)
	}
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
	var offset int

	for {
		var batch []*models.Media

		// Use Skip and Limit for pagination - note: this is a simplified approach
		// In a real scenario, you might want to use a more efficient cursor-based approach
		if err := r.store.Find(&batch, bolthold.Where("Trakt").Ge(int64(0)).Skip(offset).Limit(batchSize)); err != nil {
			return fmt.Errorf("failed to find media batch: %w", err)
		}

		if len(batch) == 0 {
			break // No more records
		}

		if err := processor(batch); err != nil {
			return fmt.Errorf("failed to process media batch: %w", err)
		}

		offset += batchSize

		// If we got fewer records than batch size, we're done
		if len(batch) < batchSize {
			break
		}
	}

	return nil
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

// NZB operations
func (r *BoltRepository) SaveNZB(nzb *models.NZB) error {
	if nzb.ID == "" {
		nzb.GenerateID()
	}
	if err := r.store.Upsert(nzb.ID, nzb); err != nil {
		return fmt.Errorf("failed to save NZB: %w", err)
	}
	return nil
}

func (r *BoltRepository) GetNZB(traktID int64) (*models.NZB, error) {
	// Try BoltDB sorting approach since it should work with proper numeric sorting
	var nzb []*models.NZB
	
	// Step 1: Look for remux files (highest priority)
	err := r.store.Find(&nzb, bolthold.Where("Trakt").Eq(traktID).And("Title").
		RegExp(regexp.MustCompile("(?i)remux")).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(1))
	if err != nil {
		return nil, fmt.Errorf("failed to get remux NZB: %w", err)
	}
	if len(nzb) > 0 {
		return nzb[0], nil
	}

	// Step 2: Look for web-dl files (medium priority)
	nzb = nil // Clear the slice
	err = r.store.Find(&nzb, bolthold.Where("Trakt").Eq(traktID).And("Title").
		RegExp(regexp.MustCompile("(?i)web-dl")).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(1))
	if err != nil {
		return nil, fmt.Errorf("failed to get web-dl NZB: %w", err)
	}
	if len(nzb) > 0 {
		return nzb[0], nil
	}

	// Step 3: Get largest file overall (fallback)
	nzb = nil // Clear the slice
	err = r.store.Find(&nzb, bolthold.Where("Trakt").Eq(traktID).
		And("Failed").Eq(false).
		SortBy("Length").Reverse().Limit(1))
	if err != nil {
		return nil, fmt.Errorf("failed to get any NZB: %w", err)
	}
	if len(nzb) > 0 {
		return nzb[0], nil
	}

	return nil, fmt.Errorf("no NZB found for Trakt ID %d", traktID)
}

func (r *BoltRepository) FindAllNZBsByTraktID(traktID int64) ([]*models.NZB, error) {
	var nzbs []*models.NZB
	if err := r.store.Find(&nzbs, bolthold.Where("Trakt").Eq(traktID)); err != nil {
		return nil, fmt.Errorf("failed to find all NZBs for Trakt ID %d: %w", traktID, err)
	}
	return nzbs, nil
}

func (r *BoltRepository) FindNZBsByTraktIDs(traktIDs []int64) ([]*models.NZB, error) {
	var nzbs []*models.NZB

	// Convert []int64 to []interface{}
	ids := make([]interface{}, len(traktIDs))
	for i, id := range traktIDs {
		ids[i] = id
	}

	if err := r.store.Find(&nzbs, bolthold.Where("Trakt").In(ids...)); err != nil {
		return nil, fmt.Errorf("failed to find NZBs by Trakt IDs: %w", err)
	}
	return nzbs, nil
}

// SaveNZBBatch saves multiple NZB items in a single transaction
func (r *BoltRepository) SaveNZBBatch(nzbs []*models.NZB) error {
	if len(nzbs) == 0 {
		return nil
	}

	tx, err := r.store.Bolt().Begin(true)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback()

	for _, nzb := range nzbs {
		if nzb.ID == "" {
			nzb.GenerateID()
		}
		if err := r.store.TxUpsert(tx, nzb.ID, nzb); err != nil {
			return fmt.Errorf("failed to save NZB in batch: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit NZB batch transaction: %w", err)
	}
	return nil
}

func (r *BoltRepository) RemoveNZBsByTraktID(traktID int64) error {
	if err := r.store.DeleteMatching(&models.NZB{}, bolthold.Where("Trakt").Eq(traktID)); err != nil {
		return fmt.Errorf("failed to remove NZBs for Trakt ID %d: %w", traktID, err)
	}
	return nil
}

// Utility operations
func (r *BoltRepository) Close() error {
	if err := r.store.Close(); err != nil {
		return fmt.Errorf("failed to close repository: %w", err)
	}
	return nil
}
