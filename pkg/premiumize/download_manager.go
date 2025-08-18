package premiumize

import (
	"context"
	"fmt"
	"time"
)

const (
	checkInterval = 10 * time.Second
)

type DownloadManager struct {
	client *Client
}

func NewDownloadManager(client *Client) *DownloadManager {
	return &DownloadManager{
		client: client,
	}
}

func (dm *DownloadManager) WaitForTransferCompletion(ctx context.Context, transferID string) (*Transfer, error) {
	ticker := time.NewTicker(checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
			transfer, err := dm.client.GetTransfer(ctx, transferID)
			if err != nil {
				return nil, fmt.Errorf("checking transfer status: %w", err)
			}

			if transfer.Status.IsComplete() {
				return transfer, nil
			}

			if transfer.Status.IsFailed() {
				return nil, fmt.Errorf("transfer failed with status: %s", transfer.Status)
			}
		}
	}
}

func (dm *DownloadManager) GetTransferLink(ctx context.Context, transfer *Transfer) (string, error) {
	folderContent, err := dm.client.ListFolder(ctx, transfer.FolderID)
	if err != nil {
		return "", fmt.Errorf("listing folder contents: %w", err)
	}

	if len(folderContent.Content.Items) == 0 {
		return "", fmt.Errorf("no files found in transfer folder")
	}

	largestFile := dm.findLargestFile(folderContent.Content.Items)
	if largestFile == nil {
		return "", fmt.Errorf("no suitable file found in transfer")
	}

	return largestFile.Link, nil
}

func (dm *DownloadManager) findLargestFile(files []File) *File {
	var largestFile *File
	var maxSize int64
	
	for i := range files {
		file := &files[i]
		if file.Size > maxSize {
			maxSize = file.Size
			largestFile = file
		}
	}
	
	return largestFile
}