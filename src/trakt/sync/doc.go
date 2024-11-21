// Package sync allows us to sync our trakt collection with one or more media centres.
//
// Syncing with trakt opens up quite a few cool features. Most importantly, trakt can serve as a cloud based
// backup for the data in your app. This is especially useful when rebuilding a media center or installing a
// mobile app on your new phone. It can also be nice to sync up multiple media centers with a central
// trakt account. If everything is in sync, your media can be managed from trakt and be reflected in your
// apps.
//
// # Media objects for syncing
//
// As a baseline, all add and remove sync methods accept arrays of movies, shows, and episodes. Each of these top
// level array elements should themselves be an array of standard movie, show, or episode objects. Full examples
// can be found at: https://trakt.docs.apiary.io/#introduction/standard-media-objects. Keep in mind that episode
// objects really only need the ids so it can find an exact match. This is useful for absolute ordered shows.
// Some methods also have optional metadata you can attach, so check the docs for each specific method.
//
// Media objects will be matched by ID first, then fall back to title and year. IDs will be matched in this order
// trakt, imdb, tmdb, tvdb, and slug. If nothing is found, it will match on the title and year. If still nothing,
// it would use just the title (or name for people) and find the most current object that exists.
//
// # Watched History Sync
//
// This is a 2 way sync that will get items from trakt to sync locally, plus find anything new and sync back to
// trakt. Perform this sync on startup or at set intervals (i.e. once every day) to keep everything in sync.
// This will only send data to trakt and not remove it.
//
// # Collection Sync
//
// It's very handy to have a snapshot on trakt of everything you have available to watch locally. Syncing your
// local connection will do just that. This will only send data to trakt and not remove it.
//
// # Clean Collection
//
// Cleaning a collection involves comparing the trakt collection to what exists locally. This will remove items
// from the trakt collection if they don't exist locally anymore. You should make this clear to the user
// that data might be removed from trakt.
package sync
