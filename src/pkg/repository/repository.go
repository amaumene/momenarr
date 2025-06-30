package repository

import (
	"fmt"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/pkg/models"
)

// Repository defines the interface for data access operations
type Repository interface {
	// Media operations
	SaveMedia(media *models.Media) error
	GetMedia(traktID int64) (*models.Media, error)
	FindMediaNotOnDisk() ([]*models.Media, error)
	UpdateMediaDownloadID(traktID, downloadID int64) error
	RemoveMedia(traktID int64) error
	FindAllMedia() ([]*models.Media, error)

	// NZB operations
	SaveNZB(nzb *models.NZB) error
	GetNZB(traktID int64) (*models.NZB, error)
	FindNZBsByTraktIDs(traktIDs []int64) ([]*models.NZB, error)
	RemoveNZBsByTraktID(traktID int64) error

	// Utility operations
	Close() error
}

// BoltRepository implements Repository using BoltDB
type BoltRepository struct {
	store *bolthold.Store
}

// NewBoltRepository creates a new BoltDB repository
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

func (r *BoltRepository) UpdateMediaDownloadID(traktID, downloadID int64) error {
	media, err := r.GetMedia(traktID)
	if err != nil {
		return fmt.Errorf("failed to get media for update: %w", err)
	}
	
	media.DownloadID = downloadID
	if err := r.store.Update(traktID, media); err != nil {
		return fmt.Errorf("failed to update media download ID: %w", err)
	}
	return nil
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
	if err := r.store.Upsert(nzb.Trakt, nzb); err != nil {
		return fmt.Errorf("failed to save NZB: %w", err)
	}
	return nil
}

func (r *BoltRepository) GetNZB(traktID int64) (*models.NZB, error) {
	var nzb models.NZB
	if err := r.store.Get(traktID, &nzb); err != nil {
		return nil, fmt.Errorf("failed to get NZB: %w", err)
	}
	return &nzb, nil
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