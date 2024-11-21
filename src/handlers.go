package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/amaumene/momenarr/trakt"
	"github.com/amaumene/momenarr/trakt/search"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"
	log "github.com/sirupsen/logrus"
	"github.com/timshannon/bolthold"
	"io"
	"os"
	"path/filepath"
)

func (appConfig *App) searchShow(showName string) ([]*trakt.Show, error) {
	params := &trakt.SearchQueryParams{
		Query: showName,
		//Type:  "show",
	}
	fmt.Printf("Searching for show: %+v\n", params)
	results := search.TextQuery(params)
	var shows []*trakt.Show
	for results.Next() {
		var result trakt.Show
		err := results.Scan(&result)
		if err != nil {
			return nil, fmt.Errorf("error getting search result entry: %v", err)
		}
		shows = append(shows, &result)
	}

	if err := results.Err(); err != nil {
		return nil, fmt.Errorf("error iterating search results: %v", err)
	}

	return shows, nil
}

func (appConfig *App) addShowToCollection(show *trakt.Show) error {
	insert := Show{
		IMDB:  string(show.IMDB),
		Title: show.Title,
	}
	err := appConfig.store.Insert(show.IMDB, insert)
	if err != nil && err.Error() != "This Key already exists in this bolthold for this type" {
		return fmt.Errorf("inserting show into database: %v", err)
	}
	return nil
}

func (appConfig *App) addMonitored(c *fiber.Ctx) error {
	if c.Method() == fiber.MethodGet {
		if err := c.Render("add", fiber.Map{}); err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse template")
		}
	}

	if c.Method() == fiber.MethodPost {
		showName := c.FormValue("show")
		if showName == "" {
			return c.Status(fiber.StatusBadRequest).SendString("Show name is required")
		}

		// Search for the show using Trakt API
		show, err := appConfig.searchShow(showName)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to search show: %v", err))
		}
		fmt.Printf("Found show: %+v\n", show)

		// Add the show to the collection
		//err = appConfig.addShowToCollection(show)
		//if err != nil {
		//	return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to add show to collection: %v", err))
		//}

		return c.Redirect("/list")
	}
	return nil
}

func (appConfig *App) listMonitored(c *fiber.Ctx) error {
	var shows []Show
	err := appConfig.store.Find(&shows, &bolthold.Query{})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to retrieve shows")
	}

	if err := c.Render("list", fiber.Map{
		"Shows": shows,
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to parse template")
	}
	return nil
}

func handleAPIRequests(appConfig *App) {
	engine := html.New("./html", ".html")
	engine.Reload(true) // Optional. Default: false
	engine.Debug(true)  // Optional. Default: false
	app := fiber.New(fiber.Config{
		Views: engine,
	})
	app.Post("/api/notify", func(c *fiber.Ctx) error {
		return handlePostData(c, *appConfig)
	})
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	app.Get("/list", func(c *fiber.Ctx) error {
		return appConfig.listMonitored(c)
	})
	app.Get("/add", func(c *fiber.Ctx) error {
		return appConfig.addMonitored(c)
	})
	app.Post("/add", func(c *fiber.Ctx) error {
		return appConfig.addMonitored(c)
	})
	app.Get("/refresh", func(c *fiber.Ctx) error {
		go func() {
			appConfig.runTasks()
		}()
		return c.SendString("Refresh initiated")
	})

	app.Listen(":3000")
}

func handlePostData(c *fiber.Ctx, appConfig App) error {
	if c.Method() != fiber.MethodPost {
		return c.Status(fiber.StatusMethodNotAllowed).SendString("Invalid request method")
	}

	body, err := io.ReadAll(bytes.NewReader(c.Body()))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read request body")
	}

	var notification Notification
	err = json.Unmarshal(body, &notification)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Failed to parse JSON")
	}
	go func() {
		err := processNotification(notification, appConfig)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("processing notification")
		}
	}()

	return c.JSON(fiber.Map{"message": "Data received and processing started"})
}

func processNotification(notification Notification, appConfig App) error {
	if notification.Category == "momenarr" && notification.Status == "SUCCESS" {
		var media Media
		err := appConfig.store.Get(notification.IMDB, &media)
		if err != nil {
			return fmt.Errorf("finding media: %v", err)
		}
		file, err := findBiggestFile(notification.Dir)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Finding biggest file")
		}

		destPath := filepath.Join(appConfig.downloadDir, filepath.Base(file))
		err = os.Rename(file, destPath)
		if err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Moving file to download directory")
		}
		media.File = file
		media.OnDisk = true
		if err := appConfig.store.Update(notification.IMDB, &media); err != nil {
			log.WithFields(log.Fields{"err": err}).Error("Update media path/status in database")
		}

		IDs := []int64{
			media.downloadID,
		}
		result, err := appConfig.nzbget.EditQueue("HistoryFinalDelete", "", IDs)
		if err != nil || result == false {
			log.WithFields(log.Fields{"err": err}).Error("Failed to delete NZBGet download")
		}
	}
	return nil
}

// findBiggestFile finds the biggest file in the given directory and its subdirectories.
func findBiggestFile(dir string) (string, error) {
	var biggestFile string
	var maxSize int64

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && info.Size() > maxSize {
			biggestFile = path
			maxSize = info.Size()
		}
		return nil
	})

	if err != nil {
		return "", err
	}

	return biggestFile, nil
}
