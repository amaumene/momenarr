package services

import (
	"fmt"
	"strconv"

	"github.com/amaumene/gostremiofr/pkg/alldebrid"
	log "github.com/sirupsen/logrus"
)

type AllDebridService struct {
	client *alldebrid.Client
	apiKey string
}

func NewAllDebridService(client *alldebrid.Client, apiKey string) *AllDebridService {
	return &AllDebridService{
		client: client,
		apiKey: apiKey,
	}
}

func (s *AllDebridService) IsTorrentCached(hash string) (bool, int64, error) {
	magnetURL := fmt.Sprintf("magnet:?xt=urn:btih:%s", hash)

	uploadResult, err := s.uploadMagnet(magnetURL)
	if err != nil {
		return false, 0, err
	}

	return s.checkMagnetStatus(uploadResult)
}

func (s *AllDebridService) uploadMagnet(magnetURL string) (*alldebrid.MagnetUploadResponse, error) {
	uploadResult, err := s.client.UploadMagnet(s.apiKey, []string{magnetURL})
	if err != nil {
		return nil, fmt.Errorf("failed to upload magnet: %w", err)
	}

	if uploadResult.Error != nil {
		return nil, fmt.Errorf("upload error: %s", uploadResult.Error.Message)
	}

	return uploadResult, nil
}

func (s *AllDebridService) checkMagnetStatus(uploadResult *alldebrid.MagnetUploadResponse) (bool, int64, error) {
	if len(uploadResult.Data.Magnets) == 0 {
		return false, 0, nil
	}

	magnet := &uploadResult.Data.Magnets[0]
	if magnet.Error != nil {
		return false, 0, fmt.Errorf("magnet error: %s", magnet.Error.Message)
	}

	if magnet.Ready {
		return true, int64(magnet.ID), nil
	}

	s.deleteNonCachedMagnet(magnet.ID)
	return false, 0, nil
}

func (s *AllDebridService) deleteNonCachedMagnet(magnetID int64) {
	if err := s.client.DeleteMagnet(s.apiKey, strconv.FormatInt(magnetID, 10)); err != nil {
		log.WithError(err).Error("Failed to delete non-cached magnet")
	}
}
