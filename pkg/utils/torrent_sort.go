package utils

import (
	"github.com/amaumene/momenarr/pkg/models"
)

// SortTorrentResultsByQuality sorts torrents by quality priority:
// 1. Remux first (highest quality)
// 2. Resolution (higher is better)
// 3. Size (larger is better)
func SortTorrentResultsByQuality(results []models.TorrentSearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if shouldSwapByQuality(results[i], results[j]) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// shouldSwapByQuality determines if torrent j should be ranked higher than torrent i
func shouldSwapByQuality(i, j models.TorrentSearchResult) bool {
	iIsRemux := i.IsRemux()
	jIsRemux := j.IsRemux()

	// 1. Remux always wins over non-remux
	if jIsRemux && !iIsRemux {
		return true
	}
	if iIsRemux && !jIsRemux {
		return false
	}

	// 2. If both are remux or both are not remux, compare by resolution
	iResolution := i.ExtractResolution()
	jResolution := j.ExtractResolution()

	if jResolution > iResolution {
		return true
	}
	if iResolution > jResolution {
		return false
	}

	// 3. If same remux status and resolution, larger size wins
	if j.Size > i.Size {
		return true
	}

	return false
}

