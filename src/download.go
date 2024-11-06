package main

import (
	"context"
	"fmt"
	"github.com/amaumene/momenarr/got"
	"github.com/amaumene/momenarr/torbox"
	"path/filepath"
)

func downloadWithGot(fileLink string, UsenetDownload []torbox.UsenetDownload, appConfig App) error {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	download := got.NewDownload(ctx, fileLink, filepath.Join(appConfig.downloadDir, UsenetDownload[0].Files[0].ShortName))
	download.Concurrency = 4
	download.ChunkSize = 1073741824 // 1GiB
	download.Interval = 10000       // 10 sec

	got.UserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15"

	if err := download.Init(); err != nil {
		fmt.Println(err)
	}
	go func() {
		if err := download.Start(); err != nil {
			fmt.Println(err)
		}
	}()

	download.RunProgress(func(d *got.Download) {
		fmt.Printf("Downloading %s: %d MB/s / %d MB\n",
			UsenetDownload[0].Files[0].ShortName,
			download.AvgSpeed()/1024/1024,
			download.Size()/1024/1024,
		)
	})

	cancel()
	fmt.Println("Download started")
	return nil
}

func downloadFromTorBox(UsenetDownload []torbox.UsenetDownload, appConfig App) error {
	fileLink, err := appConfig.TorBoxClient.RequestUsenetDownloadLink(UsenetDownload)
	if err != nil {
		return err
	}
	err = downloadWithGot(fileLink, UsenetDownload, appConfig)
	if err != nil {
		return err
	}
	fmt.Println("Download finished")
	return nil
}
