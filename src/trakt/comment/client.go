package comment

import (
	"net/http"

	"github.com/jacklaaa89/trakt"
)

// client represents a client which is capable of perform comment
// based operations, utilising the base client.
type client struct{ b trakt.BaseClient }

// Get returns a single comment and indicates how many replies it has. Use "Replies" to get the actual replies.
func Get(id int64, params *trakt.BasicParams) (*trakt.Comment, error) { return getC().Get(id, params) }

// Get returns a single comment and indicates how many replies it has. Use "Replies" to get the actual replies.
func (c *client) Get(id int64, params *trakt.BasicParams) (*trakt.Comment, error) {
	path := trakt.FormatURLPath("/comments/%s", id)
	com := &trakt.Comment{}
	err := c.b.Call(http.MethodGet, path, params, com)
	return com, err
}

// Likes returns all users who liked a comment. If you only need the replies count, the main comment
// object already has that, so no need to use this method.
//
//   - Pagination
func Likes(id int64, p *trakt.BasicListParams) *trakt.UserLikeIterator { return getC().Likes(id, p) }

// Likes returns all users who liked a comment. If you only need the replies count, the main comment
// object already has that, so no need to use this method.
//
//   - Pagination
func (c *client) Likes(id int64, params *trakt.BasicListParams) *trakt.UserLikeIterator {
	path := trakt.FormatURLPath("/comments/%s/likes", id)
	return &trakt.UserLikeIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Replies returns all replies for a comment. It is possible these replies could have replies themselves,
// so in that case you would just call this function again with the new comment id.
//
//   - Pagination
func Replies(id int64, p *trakt.ListParams) *trakt.CommentIterator { return getC().Replies(id, p) }

// Replies returns all replies for a comment. It is possible these replies could have replies themselves,
// so in that case you would just call this function again with the new comment id.
//
//   - Pagination
func (c *client) Replies(id int64, params *trakt.ListParams) *trakt.CommentIterator {
	path := trakt.FormatURLPath("/comments/%s/replies", id)
	return &trakt.CommentIterator{Iterator: c.b.NewIterator(http.MethodGet, path, params)}
}

// Item returns the media item this comment is attached to. The media type can be movie,
// show, season, episode, or list and it also returns the standard media object for that media type.
//
//   - Extended Info
func Item(id int64, p *trakt.ExtendedParams) (*trakt.GenericElement, error) {
	return getC().Item(id, p)
}

// Item returns the media item this comment is attached to. The media type can be movie,
// show, season, episode, or list and it also returns the standard media object for that media type.
//
//   - Extended Info
func (c *client) Item(id int64, params *trakt.ExtendedParams) (*trakt.GenericElement, error) {
	path := trakt.FormatURLPath("/comments/%s/item", id)
	com := &trakt.GenericElement{}
	err := c.b.Call(http.MethodGet, path, params, com)
	return com, err
}

// Trending returns all comments with the most likes and replies over the last 7 days. You can optionally filter
// by the comment_type and media type to limit what gets returned.
//
//   - Pagination
//   - Extended Info
func Trending(p *trakt.TrendingCommentParams) *trakt.CommentWithMediaElementIterator {
	return getC().Trending(p)
}

// Trending returns all comments with the most likes and replies over the last 7 days. You can optionally filter
// by the comment_type and media type to limit what gets returned.
//
//   - Pagination
//   - Extended Info
func (c *client) Trending(params *trakt.TrendingCommentParams) *trakt.CommentWithMediaElementIterator {
	return c.generateIterator(`trending`, params)
}

// Recent returns the most recently written comments across all of Trakt. You can optionally filter
// by the comment_type and media type to limit what gets returned.
//
//   - Pagination
//   - Extended Info
func Recent(params *trakt.RecentCommentParams) *trakt.CommentWithMediaElementIterator {
	return getC().Recent(params)
}

// Recent returns the most recently written comments across all of Trakt. You can optionally filter
// by the comment_type and media type to limit what gets returned.
//
//   - Pagination
//   - Extended Info
func (c *client) Recent(params *trakt.RecentCommentParams) *trakt.CommentWithMediaElementIterator {
	return c.generateIterator(`recent`, params)
}

// Updates returns the most recently updated comments across all of Trakt. You can optionally filter
// by the comment_type and media type to limit what gets returned.
//
//   - Pagination
//   - Extended Info
func Updates(params *trakt.UpdatedCommentParams) *trakt.CommentWithMediaElementIterator {
	return getC().Updates(params)
}

// Updates returns the most recently updated comments across all of Trakt. You can optionally filter
// by the comment_type and media type to limit what gets returned.
//
//   - Pagination
//   - Extended Info
func (c *client) Updates(params *trakt.UpdatedCommentParams) *trakt.CommentWithMediaElementIterator {
	return c.generateIterator(`updates`, params)
}

// Post attempt to add a new comment to a movie, show, season, episode, or list. Make sure to allow and
// encourage spoilers to be indicated in your app. We need to make sure that rules outlined are followed,
// which are:
//
//   - Comments must be at least 5 words.
//   - Comments 200 words or longer will be automatically marked as a review.
//   - Correctly indicate if the comment contains spoilers.
//   - Only write comments in English - This is important!
//   - Do not include app specific text like (via App Name) or #apphashtag. This clutters up the comments and
//     failure to clean the comment text could get your app blacklisted from commenting.
//
// This function will return the following error responses:
//
// ErrorCodePostInvalidUser	       - the user (oauth token) is invalid or the user has been banned from commenting.
// ErrorCodePostInvalidItem        - item not found or doesn't allow comments
// ErrorCodeCommentCannotBeRemoved - comment can't be deleted
// ErrorCodeValidationError        - comment does not conform to rules set out above.
//
//   - OAuth Required
func Post(p *trakt.PostCommentParams) (*trakt.Comment, error) { return getC().Post(p) }

// Post attempt to add a new comment to a movie, show, season, episode, or list. Make sure to allow and
// encourage spoilers to be indicated in your app. We need to make sure that rules outlined are followed,
// which are:
//
//   - Comments must be at least 5 words.
//   - Comments 200 words or longer will be automatically marked as a review.
//   - Correctly indicate if the comment contains spoilers.
//   - Only write comments in English - This is important!
//   - Do not include app specific text like (via App Name) or #apphashtag. This clutters up the comments and
//     failure to clean the comment text could get your app blacklisted from commenting.
//
// This function will return the following error responses:
//
// "ErrorCodePostInvalidUser"	       - the user (oauth token) is invalid or the user has been banned from commenting.
// "ErrorCodePostInvalidItem"        - item not found or doesn't allow comments
// "ErrorCodeCommentCannotBeRemoved" - comment can't be deleted
// "ErrorCodeValidationError"        - comment does not conform to rules set out above.
//
//   - OAuth Required
func (c *client) Post(params *trakt.PostCommentParams) (*trakt.Comment, error) {
	com := &trakt.Comment{}
	err := c.b.Call(http.MethodPost, "/comments", &wrappedPostCommentParams{PostCommentParams: params}, &com)
	return com, err
}

// Update attempts to update a single comment. The OAuth user must match the author of the
// comment in order to update it. If not, an error with the "ErrorCodePostInvalidUser" error code
// is returned.
//
//   - OAuth Required
func Update(id int64, p *trakt.UpdateCommentParams) (*trakt.Comment, error) {
	return getC().Update(id, p)
}

// Update attempts to update a single comment. The OAuth user must match the author of the
// comment in order to update it. If not, an error with the "ErrorCodePostInvalidUser" error code
// is returned.
//
//   - OAuth Required
func (c *client) Update(id int64, params *trakt.UpdateCommentParams) (*trakt.Comment, error) {
	com := &trakt.Comment{}
	err := c.b.Call(
		http.MethodPut,
		trakt.FormatURLPath("/comments/%s", id),
		&wrappedUpdateCommentParams{UpdateCommentParams: params},
		&com,
	)
	return com, err
}

// Remove attempts to delete a single comment. The OAuth user must match the author of the comment
// in order to delete it. If not, an error with the "ErrorCodePostInvalidUser" error code is returned.
// The comment must also be less than 2 weeks old or have 0 replies. If not, an error with the
// "ErrorCodeCommentCannotBeRemoved" error code is returned.
//
//   - OAuth Required
func Remove(id int64, p *trakt.Params) error { return getC().Remove(id, p) }

// Remove attempts to delete a single comment. The OAuth user must match the author of the comment
// in order to delete it. If not, an error with the "ErrorCodePostInvalidUser" error code is returned.
// The comment must also be less than 2 weeks old or have 0 replies. If not, an error with the
// "ErrorCodeCommentCannotBeRemoved" error code is returned.
//
//   - OAuth Required
func (c *client) Remove(id int64, params *trakt.Params) error {
	return c.b.Call(
		http.MethodDelete, trakt.FormatURLPath("/comment/%s", id),
		&wrappedRemoveCommentParams{Params: params}, nil,
	)
}

// AddReply attempts to add a new reply to an existing comment. Make sure to allow and encourage
// spoilers to be indicated in your app and follow the rules listed above.
//
//   - OAuth Required
func AddReply(id int64, p *trakt.AddReplyParams) (*trakt.Comment, error) {
	return getC().AddReply(id, p)
}

// AddReply attempts to add a new reply to an existing comment. Make sure to allow and encourage
// spoilers to be indicated in your app and follow the rules listed above.
//
//   - OAuth Required
func (c *client) AddReply(id int64, params *trakt.AddReplyParams) (*trakt.Comment, error) {
	com := &trakt.Comment{}
	err := c.b.Call(
		http.MethodPost,
		trakt.FormatURLPath("/comments/%s/replies", id),
		&wrappedUpdateCommentParams{UpdateCommentParams: params},
		&com,
	)
	return com, err
}

// AddLike attempts to add a like to a comment.
// Votes help determine popular comments. Only one like is allowed per comment per user.
//
//   - OAuth Required
func AddLike(id int64, p *trakt.Params) error { return getC().AddLike(id, p) }

// AddLike attempts to add a like to a comment.
// Votes help determine popular comments. Only one like is allowed per comment per user.
//
//   - OAuth Required
func (c *client) AddLike(id int64, params *trakt.Params) error {
	return c.b.Call(http.MethodPost, trakt.FormatURLPath("/comments/%s/like", id), params, nil)
}

// RemoveLike attempts to remove as like on a comment.
//
//   - OAuth Required
func RemoveLike(id int64, p *trakt.Params) error { return getC().RemoveLike(id, p) }

// RemoveLike attempts to remove as like on a comment.
//
//   - OAuth Required
func (c *client) RemoveLike(id int64, params *trakt.Params) error {
	return c.b.Call(http.MethodDelete, trakt.FormatURLPath("/comments/%s/like", id), params, nil)
}

// wrappedPostCommentParams wrapped post comment params which
// attaches the custom error handler required for posting comments.
type wrappedPostCommentParams struct {
	commentErrorHandler
	*trakt.PostCommentParams
}

// wrappedUpdateCommentParams wrapped post comment params which
// attaches the custom error handler required for updating comments
// or adding replies.
type wrappedUpdateCommentParams struct {
	commentErrorHandler
	*trakt.UpdateCommentParams
}

// wrappedRemoveCommentParams wrapped post comment params which
// attaches the custom error handler required for removing comments.
type wrappedRemoveCommentParams struct {
	commentErrorHandler
	*trakt.Params
}

// commentErrorHandler a structure which has a custom
// error handler attached which handles errors from adding
// or removing replies or comments.
type commentErrorHandler struct{}

// Code implements ErrorHandler interface.
func (commentErrorHandler) Code(statusCode int) trakt.ErrorCode {
	switch statusCode {
	case http.StatusUnauthorized:
		return trakt.ErrorCodePostInvalidUser
	case http.StatusNotFound:
		return trakt.ErrorCodePostInvalidItem
	case http.StatusConflict:
		return trakt.ErrorCodeCommentCannotBeRemoved
	}

	return trakt.DefaultErrorHandler.Code(statusCode)
}

// generateIterator generates an iterator for the functions
// - Trending
// - Recent
// - Updates
// as the only thing that changes is the action that is called in terms of arguments.
func (c *client) generateIterator(act string, p *trakt.TrendingCommentParams) *trakt.CommentWithMediaElementIterator {
	var ct, mt = trakt.All, trakt.All
	if p.MediaType != "" {
		mt = string(p.MediaType)
	}
	if p.CommentType != "" {
		mt = string(p.CommentType)
	}

	path := trakt.FormatURLPath("/comments/%s/%s/%s", act, ct, mt)
	return &trakt.CommentWithMediaElementIterator{Iterator: c.b.NewIterator(http.MethodGet, path, p)}
}

// getC initialises a new comment client from the current backend configuration.
func getC() *client { return &client{trakt.NewClient()} }
