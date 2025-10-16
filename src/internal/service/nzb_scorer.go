package service

import "strings"

const (
	maxResolutionScore = 40
	maxSourceScore     = 30
	maxCodecScore      = 20
	maxFlagsScore      = 10
)

const (
	resolution2160p = "2160P"
	resolution4K    = "4K"
	resolution1080p = "1080P"
	resolution720p  = "720P"
	resolution576p  = "576P"
	resolution480p  = "480P"
)

const (
	sourceREMUX  = "REMUX"
	sourceBLURAY = "BLURAY"
	sourceBDRIP  = "BDRIP"
	sourceWEBDL  = "WEB-DL"
	sourceWEBRIP = "WEBRIP"
	sourceHDTV   = "HDTV"
)

const (
	codecX265 = "X265"
	codecHEVC = "HEVC"
	codecX264 = "X264"
	codecAVC  = "AVC"
	codecXVID = "XVID"
)

func scoreQuality(parsed *ParsedNZB) int {
	resScore := scoreResolution(parsed.Resolution)
	srcScore := scoreSource(parsed.Source)
	codecScore := scoreCodec(parsed.Codec)
	flagsScore := scoreFlags(parsed.Proper, parsed.Repack)

	return resScore + srcScore + codecScore + flagsScore
}

func scoreResolution(resolution string) int {
	normalized := strings.ToUpper(resolution)

	switch {
	case contains(normalized, resolution2160p) || contains(normalized, resolution4K):
		return 40
	case contains(normalized, resolution1080p):
		return 30
	case contains(normalized, resolution720p):
		return 20
	case contains(normalized, resolution576p) || contains(normalized, resolution480p):
		return 10
	default:
		return 5
	}
}

func scoreSource(source string) int {
	normalized := strings.ToUpper(source)

	switch {
	case contains(normalized, sourceREMUX):
		return 30
	case contains(normalized, sourceBLURAY) || contains(normalized, sourceBDRIP):
		return 25
	case contains(normalized, sourceWEBDL):
		return 20
	case contains(normalized, sourceWEBRIP):
		return 15
	case contains(normalized, sourceHDTV):
		return 10
	default:
		return 5
	}
}

func scoreCodec(codec string) int {
	normalized := strings.ToUpper(codec)

	switch {
	case contains(normalized, codecX265) || contains(normalized, codecHEVC):
		return 20
	case contains(normalized, codecX264) || contains(normalized, codecAVC):
		return 15
	case contains(normalized, codecXVID):
		return 10
	default:
		return 5
	}
}

func scoreFlags(proper, repack bool) int {
	score := 0
	if proper {
		score += 5
	}
	if repack {
		score += 5
	}
	if score > maxFlagsScore {
		return maxFlagsScore
	}
	return score
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
