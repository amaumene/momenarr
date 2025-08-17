package utils

import (
	"sort"

	"github.com/amaumene/momenarr/pkg/models"
)


func SortTorrentResultsByQuality(results []models.TorrentSearchResult) {
	sort.Slice(results, func(i, j int) bool {
		return isHigherQuality(results[j], results[i])
	})
}

func isHigherQuality(a, b models.TorrentSearchResult) bool {
	aIsRemux := a.IsRemux()
	bIsRemux := b.IsRemux()

	if aIsRemux != bIsRemux {
		return aIsRemux
	}

	aResolution := a.ExtractResolution()
	bResolution := b.ExtractResolution()

	if aResolution != bResolution {
		return aResolution > bResolution
	}

	return a.Size > b.Size
}
