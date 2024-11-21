// Package comment gives us functionality to post, update and list comments.
//
// Comments are attached to any movie, show, season, episode, or list and can be a quick shout or a more
// detailed review. Each comment can have replies and can be liked. These likes are used to determine popular
// comments. Comments must follow these rules and your app should indicate these to the user. Failure to
// adhere to these rules could suspend the user's commenting abilities.
//
// - Comments must be at least 5 words.
// - Comments 200 words or longer will be automatically marked as a review.
// - Correctly indicate if the comment contains spoilers.
// - Only write comments in English - This is important!
// - Do not include app specific text like (via App Name) or #apphashtag. This clutters up the comments and
// failure to clean the comment text could get your app blacklisted from commenting.
//
// Comment Formatting.
//
// Comments support markdown formatting so you'll want to render this in your app so it matches what the
// website does. In addition, we support inline spoiler tags like [spoiler]text[/spoiler] which you should
// also handle independent of the top level spoiler attribute.
//
// 😁 Emojis.
//
// We use short codes for emojis like :smiley: and :raised_hands: and render them on the Trakt website using
// EmojiOne. The Trakt API accepts standard unicode emojis, but we'll auto convert them to short codes
// before saving the comment. For the best compatibility, we recommend having your app do this conversion,
// just to make sure the correct emoji is used.
package comment
