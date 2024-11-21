package trakt

import (
	"encoding/json"
	"strings"
	"time"
)

// CommentType represents the type of comment it is.
type CommentType string

const (
	// CommentTypeUnspecified where we dont know what the comment type is.
	CommentTypeUnspecified CommentType = `unspecified`
	// CommentTypeReview is where a review is more than 200 words long.
	CommentTypeReview CommentType = `review`
	// CommentTypeShout is where a review is less than 200 words.
	CommentTypeShout CommentType = `shout`
	// CommentTypeAll is used for filtering where we want to retrieve all
	// comment types.
	CommentTypeAll = CommentType(All)
)

// PostCommentParams parameters in order to post a new comment.
type PostCommentParams struct {
	// Params is the basic parameters which all requests can take where
	// OAuth is required.
	Params

	// Type the type of media element. Can either be TypeMovie or TypeEpisode.
	Type Type `json:"-"`
	// Element the actual element data. We can provide as much or as little as
	// we want about the item. The recommended values are either the trakt ID or slug.
	Element *GenericElementParams `json:"-"`

	// Text for the comment. This supports markdown and emojis.
	// Emojis are declared as short codes like :smiley: and :raised_hands:.
	Text string `json:"comment"`
	// Spoiler represents whether the comment is a spoiler to the attached item.
	Spoiler bool `json:"spoiler"`

	// The sharing object is optional and will apply the user's settings if not sent.
	// If sharing is sent, each key will override the user's setting for that social network.
	// Send true to post or false to not post on the indicated social network. You can see which
	// social networks a user has connected with the /users/settings method.
	Sharing *SharingParams `json:"sharing,omitempty"`
}

// MarshalJSON implements the MarshalJSON interface.
func (p *PostCommentParams) MarshalJSON() ([]byte, error) {
	m := marshalToMap(p)
	m[strings.ToLower(string(p.Type))] = p.Element
	return json.Marshal(m)
}

// UpdateCommentParams represents parameters which are required
// to update an existing comment.
type UpdateCommentParams struct {
	// Params is the basic parameters which all requests can take where
	// OAuth is required.
	Params

	// Text the updated comment text.
	Text string `json:"comment"`
	// Spoiler whether this comment contains a spoiler for the item
	// its commenting on.
	Spoiler bool `json:"spoiler"`
}

// AddReplyParams represents parameters which are required
// to add a reply to a comment.
type AddReplyParams = UpdateCommentParams

// TrendingCommentParams represents parameters required to
// retrieve a list of trending comments.
type TrendingCommentParams struct {
	// BasicListParams is the parameters which all requests can take for listing
	// based operations where no OAuth token is required.
	BasicListParams

	// Extended sets the level of detail required
	Extended ExtendedType `url:"extended,omitempty" json:"-"`
	// CommentType the type of comments to filter by.
	CommentType CommentType `url:"-"  json:"-"`
	// MediaType the type of media item to filter by.
	MediaType Type `url:"-"  json:"-"`
}

// RecentCommentParams represents parameters required to
// retrieve a list of the most recent comments.
type RecentCommentParams = TrendingCommentParams

// UpdatedCommentParams represents parameters required to
// retrieve a list of the recently updated comments.
type UpdatedCommentParams = TrendingCommentParams

// CommentListParams represents the parameters required
// to retrieve the comments against a single item.
type CommentListParams struct {
	// BasicListParams is the parameters which all requests can take for listing
	// based operations where no OAuth token is required.
	BasicListParams

	// Sort how to sort the list of results.
	Sort SortType `json:"-" url:"-"`
}

// Comment represents a single comment or reply.
type Comment struct {
	// ID the uuid of the comment.
	ID int `json:"id"`
	// Text the comment text or content.
	Text string `json:"comment"`
	// Spoiler whether this comment is a spoiler for
	// the item it is attached to.
	Spoiler bool `json:"spoiler"`
	// Review whether this comment is classed as a review.
	Review bool `json:"review"`
	// Parent the id of the parent comment if this is a reply.
	Parent int `json:"parent_id"`
	// CreatedAt when the comment was created
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt when the comment was last updated.
	UpdatedAt time.Time `json:"updated_at"`
	// Replies the number of replies this comment has.
	Replies int `json:"replies"`
	// Likes the number of likes.
	Likes int `json:"likes"`
	// User the user who wrote the comment.
	User *User `json:"user"`
}

// CommentIterator represents a list of comments which can be iterated.
type CommentIterator struct{ Iterator }

// Comment attempts to return an Comment entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (li *CommentIterator) Comment() (*Comment, error) {
	rcv := &Comment{}
	return rcv, li.Scan(rcv)
}

// UserLike represents a user which has liked a comment
type UserLike struct {
	// User the user who liked the reply
	User `json:"user"`
	// LikedAt the time at which the user liked the comment.
	LikedAt time.Time `json:"liked_at"`
}

// UserLikeIterator represents a list of UserLikes which can be iterated.
type UserLikeIterator struct{ Iterator }

// UserLike attempts to return an UserLike entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (li *UserLikeIterator) UserLike() (*UserLike, error) {
	rcv := &UserLike{}
	return rcv, li.Scan(rcv)
}

// CommentWithMediaElement represents a comment with the media element its attached to
type CommentWithMediaElement struct {
	// GenericMediaElement the media element the comment is attached to.
	GenericMediaElement
	// Comment the comment.
	Comment *Comment `json:"comment"`
}

// CommentWithMediaElementIterator represents a list of comments which can be iterated with the
// media element attached.
type CommentWithMediaElementIterator struct{ Iterator }

// CommentWithMediaElement attempts to return an CommentWithMediaElement entry at the current cursor in
// the iterator. Returns an error if there no cursor (Next hasnt been called yet)
// or if there is an error on the iterator retrieving a page of results.
func (li *CommentWithMediaElementIterator) CommentWithMediaElement() (*CommentWithMediaElement, error) {
	rcv := &CommentWithMediaElement{}
	return rcv, li.Scan(rcv)
}
