package clients

import (
	"context"
	"fmt"
	"strconv"

	"github.com/amaumene/momenarr/internal/domain"
	"github.com/amaumene/momenarr/nzbget"
)

const (
	historyDeleteCommand = "HistoryFinalDelete"
	decimalBase          = 10
)

type nzbgetAdapter struct {
	client *nzbget.NZBGet
}

func NewNZBGetAdapter(client *nzbget.NZBGet) domain.DownloadClient {
	return &nzbgetAdapter{client: client}
}

func (a *nzbgetAdapter) Append(ctx context.Context, input *domain.DownloadInput) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	nzbInput := convertToNZBGetInput(input)
	downloadID, err := a.client.Append(nzbInput)
	if err != nil {
		return 0, fmt.Errorf("appending to nzbget: %w", err)
	}
	return downloadID, nil
}

func (a *nzbgetAdapter) ListGroups(ctx context.Context) ([]domain.QueueItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	groups, err := a.client.ListGroups()
	if err != nil {
		return nil, fmt.Errorf("listing groups: %w", err)
	}

	return convertFromNZBGetGroups(groups), nil
}

func (a *nzbgetAdapter) History(ctx context.Context, includeHidden bool) ([]domain.HistoryItem, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	history, err := a.client.History(includeHidden)
	if err != nil {
		return nil, fmt.Errorf("getting history: %w", err)
	}

	return convertFromNZBGetHistory(history), nil
}

func (a *nzbgetAdapter) DeleteFromHistory(ctx context.Context, downloadID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	ids := []int64{downloadID}
	result, err := a.client.EditQueue(historyDeleteCommand, "", ids)
	if err != nil || !result {
		return fmt.Errorf("deleting from history: %w", err)
	}
	return nil
}

func convertToNZBGetInput(input *domain.DownloadInput) *nzbget.AppendInput {
	params := make([]*nzbget.Parameter, 0, len(input.Parameters))
	for key, value := range input.Parameters {
		params = append(params, &nzbget.Parameter{Name: key, Value: value})
	}

	return &nzbget.AppendInput{
		Filename:     input.Filename,
		Content:      input.Content,
		Category:     input.Category,
		DupeMode:     input.DupeMode,
		AutoCategory: false,
		Parameters:   params,
	}
}

func convertFromNZBGetGroups(groups []*nzbget.Group) []domain.QueueItem {
	items := make([]domain.QueueItem, len(groups))
	for i, group := range groups {
		items[i] = domain.QueueItem{
			NZBID:   group.NZBID,
			NZBName: group.NZBName,
		}
	}
	return items
}

func convertFromNZBGetHistory(history []*nzbget.History) []domain.HistoryItem {
	items := make([]domain.HistoryItem, len(history))
	for i, item := range history {
		items[i] = domain.HistoryItem{
			NZBID: item.NZBID,
		}
	}
	return items
}

func formatTraktID(traktID int64) string {
	return strconv.FormatInt(traktID, decimalBase)
}
