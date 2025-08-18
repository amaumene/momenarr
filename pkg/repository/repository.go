package repository

import (
	"context"
	"fmt"
	"regexp"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/pkg/models"
)

// Pre-compiled regex patterns for better performance
var (
	remuxRegex = regexp.MustCompile("(?i)remux")
	webDLRegex = regexp.MustCompile("(?i)web-dl")
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
	GetMediaByDownloadID(downloadID int64) (*models.Media, error)
	RemoveMedia(traktID int64) error
	FindAllMedia() ([]*models.Media, error)

	// NZB operations
	SaveNZB(nzb *models.NZB) error
	SaveNZBBatch(nzbs []*models.NZB) error
	GetNZB(traktID int64) (*models.NZB, error)
	FindAllNZBsByTraktID(traktID int64) ([]*models.NZB, error)
	FindNZBsByTraktIDs(traktIDs []int64) ([]*models.NZB, error)
	RemoveNZBsByTraktID(traktID int64) error

	// Season Pack operations
	SaveSeasonPack(pack *models.SeasonPack) error
	GetSeasonPack(showTMDBID, season int64) (*models.SeasonPack, error)
	RemoveSeasonPack(packID int64) error
	GetEpisodesBySeason(showTMDBID, season int64) ([]*models.Media, error)

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
		batch, hasMore, err := r.fetchMediaBatch(ctx, lastID, batchSize)
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
		if !hasMore {
			break
		}
	}
	return nil
}

func (r *BoltRepository) fetchMediaBatch(ctx context.Context, lastID int64, batchSize int) ([]*models.Media, bool, error) {
	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	default:
	}

	var batch []*models.Media
	if err := r.store.Find(&batch, bolthold.Where("Trakt").Gt(lastID).SortBy("Trakt").Limit(batchSize)); err != nil {
		return nil, false, fmt.Errorf("failed to find media batch: %w", err)
	}

	hasMore := len(batch) == batchSize
	return batch, hasMore, nil
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

func (r *BoltRepository) GetMediaByDownloadID(downloadID int64) (*models.Media, error) {
	var media models.Media
	if err := r.store.FindOne(&media, bolthold.Where("DownloadID").Eq(downloadID)); err != nil {
		return nil, fmt.Errorf("failed to get media by download ID: %w", err)
	}
	return &media, nil
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
	nzbs, err := r.findNonFailedNZBs(traktID)
	if err != nil {
		return nil, err
	}
	if len(nzbs) == 0 {
		return nil, fmt.Errorf("no NZB found for Trakt ID %d", traktID)
	}
	return r.selectBestNZB(nzbs), nil
}

func (r *BoltRepository) findNonFailedNZBs(traktID int64) ([]*models.NZB, error) {
	var nzbs []*models.NZB
	err := r.store.Find(&nzbs, bolthold.Where("Trakt").Eq(traktID).
		And("Failed").Eq(false))
	if err != nil {
		return nil, fmt.Errorf("failed to get NZBs: %w", err)
	}
	return nzbs, nil
}

func (r *BoltRepository) selectBestNZB(nzbs []*models.NZB) *models.NZB {
	var bestNZB *models.NZB
	var bestScore int

	for _, nzb := range nzbs {
		score := r.scoreNZB(nzb)
		if score > bestScore {
			bestScore = score
			bestNZB = nzb
		}
	}
	return bestNZB
}

func (r *BoltRepository) scoreNZB(nzb *models.NZB) int {
	score := r.getQualityScore(nzb.Title)
	score += r.getSizeScore(nzb.Length)
	return score
}

func (r *BoltRepository) getQualityScore(title string) int {
	if remuxRegex.MatchString(title) {
		return 3000000000
	}
	if webDLRegex.MatchString(title) {
		return 2000000000
	}
	return 1000000000
}

func (r *BoltRepository) getSizeScore(length int64) int {
	if length < 1000000000 {
		return int(length)
	}
	return 999999999
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
	committed := false
	defer func() {
		if !committed {
			tx.Rollback()
		}
	}()
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
	committed = true
	return nil
}

func (r *BoltRepository) RemoveNZBsByTraktID(traktID int64) error {
	if err := r.store.DeleteMatching(&models.NZB{}, bolthold.Where("Trakt").Eq(traktID)); err != nil {
		return fmt.Errorf("failed to remove NZBs for Trakt ID %d: %w", traktID, err)
	}
	return nil
}

// Season Pack operations
func (r *BoltRepository) SaveSeasonPack(pack *models.SeasonPack) error {
	if err := r.store.Upsert(pack.ID, pack); err != nil {
		return fmt.Errorf("failed to save season pack: %w", err)
	}
	return nil
}

func (r *BoltRepository) GetSeasonPack(showTMDBID, season int64) (*models.SeasonPack, error) {
	var pack models.SeasonPack
	err := r.store.FindOne(&pack, bolthold.Where("ShowTMDBID").Eq(showTMDBID).And("Season").Eq(season))
	if err != nil {
		if err == bolthold.ErrNotFound {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get season pack: %w", err)
	}
	return &pack, nil
}

func (r *BoltRepository) RemoveSeasonPack(packID int64) error {
	if err := r.store.Delete(packID, &models.SeasonPack{}); err != nil {
		return fmt.Errorf("failed to remove season pack: %w", err)
	}
	return nil
}

func (r *BoltRepository) GetEpisodesBySeason(showTMDBID, season int64) ([]*models.Media, error) {
	var episodes []*models.Media
	err := r.store.Find(&episodes, bolthold.Where("ShowTMDBID").Eq(showTMDBID).And("Season").Eq(season))
	if err != nil {
		return nil, fmt.Errorf("failed to get episodes for season: %w", err)
	}
	return episodes, nil
}

// Utility operations
func (r *BoltRepository) Close() error {
	if err := r.store.Close(); err != nil {
		return fmt.Errorf("failed to close repository: %w", err)
	}
	return nil
}
