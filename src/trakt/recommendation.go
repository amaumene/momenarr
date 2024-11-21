package trakt

type RecommendationListParams struct {
	ListParams

	IgnoreCollected bool `json:"-" url:"ignore_collected"`
}
