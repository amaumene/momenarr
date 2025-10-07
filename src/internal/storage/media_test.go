package storage

import (
	"context"
	"os"
	"testing"

	"github.com/amaumene/momenarr/bolthold"
	"github.com/amaumene/momenarr/internal/domain"
)

func setupTestStore(t *testing.T) *bolthold.Store {
	t.Helper()
	tmpfile, err := os.CreateTemp("", "test_*.db")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	tmpfile.Close()

	store, err := bolthold.Open(tmpfile.Name(), 0666, nil)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		os.Remove(tmpfile.Name())
	})

	return store
}

func TestMediaRepository_Insert(t *testing.T) {
	store := setupTestStore(t)
	repo := NewMediaRepository(store)
	ctx := context.Background()

	tests := []struct {
		name    string
		media   *domain.Media
		wantErr bool
	}{
		{
			name: "valid media",
			media: &domain.Media{
				TraktID: 12345,
				IMDB:    "tt1234567",
				Title:   "Test Movie",
				Year:    2023,
				OnDisk:  false,
			},
			wantErr: false,
		},
		{
			name: "duplicate key",
			media: &domain.Media{
				TraktID: 12345,
				IMDB:    "tt1234567",
				Title:   "Test Movie",
				Year:    2023,
				OnDisk:  false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := repo.Insert(ctx, tt.media.TraktID, tt.media)
			if (err != nil) != tt.wantErr {
				t.Errorf("Insert() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMediaRepository_Get(t *testing.T) {
	store := setupTestStore(t)
	repo := NewMediaRepository(store)
	ctx := context.Background()

	media := &domain.Media{
		TraktID: 12345,
		IMDB:    "tt1234567",
		Title:   "Test Movie",
		Year:    2023,
		OnDisk:  false,
	}

	repo.Insert(ctx, media.TraktID, media)

	tests := []struct {
		name    string
		traktID int64
		wantErr bool
	}{
		{
			name:    "existing media",
			traktID: 12345,
			wantErr: false,
		},
		{
			name:    "non-existing media",
			traktID: 99999,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.Get(ctx, tt.traktID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.TraktID != tt.traktID {
				t.Errorf("Get() TraktID = %v, want %v", got.TraktID, tt.traktID)
			}
		})
	}
}

func TestMediaRepository_FindNotOnDisk(t *testing.T) {
	store := setupTestStore(t)
	repo := NewMediaRepository(store)
	ctx := context.Background()

	medias := []*domain.Media{
		{TraktID: 1, IMDB: "tt1", Title: "Movie 1", OnDisk: false},
		{TraktID: 2, IMDB: "tt2", Title: "Movie 2", OnDisk: true},
		{TraktID: 3, IMDB: "tt3", Title: "Movie 3", OnDisk: false},
	}

	for _, m := range medias {
		repo.Insert(ctx, m.TraktID, m)
	}

	got, err := repo.FindNotOnDisk(ctx)
	if err != nil {
		t.Fatalf("FindNotOnDisk() error = %v", err)
	}

	if len(got) != 2 {
		t.Errorf("FindNotOnDisk() count = %v, want 2", len(got))
	}

	for _, m := range got {
		if m.OnDisk {
			t.Errorf("FindNotOnDisk() returned media with OnDisk = true")
		}
	}
}
