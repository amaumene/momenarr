package authorization

import (
	"errors"
	"net/http"

	"github.com/google/go-querystring/query"

	"github.com/jacklaaa89/trakt"
)

// grantType represents a grant type.
type grantType string

const (
	authorizationCode grantType = "authorization_code"
	refreshToken      grantType = "refresh_token"
)

// wrappedAuthorizeURLParams a wrapped request which takes the
// supplied input and attaches the information we already know up-front.
type wrappedAuthorizeURLParams struct {
	trakt.AuthorizationURLParams `url:",inline"`

	// ClientID this is the client id which is assigned to you when you set up an
	// application on Trakt. See: https://trakt.tv/oauth/applications/new to create a new application.
	ClientID string `url:"client_id"`
	// ResponseType this is the requested response type. When performing initial OAuth authentication
	// this is always set to "code".
	ResponseType string `url:"response_type"`
}

// AuthorizeURL constructs the authorization URL to redirect to. The Trakt website will request
// permissions for your app and the user will have the opportunity to sign up for a new Trakt
// account or sign in with their existing account.
//
// Redirect URI is required, this is the URI which Trakt will redirect to after the authorization flow is
// complete. You can use "urn:ietf:wg:oauth:2.0:oob" for device authentication.
// The state is a random unique string which will be returned to the Redirect URI. This can be compared against
// to reduce the risk of "Man-In-The-Middle" attacks. See: https://en.wikipedia.org/wiki/Man-in-the-middle_attack
func AuthorizeURL(params *trakt.AuthorizationURLParams) (string, error) {
	return getC().AuthorizeURL(params)
}

// AuthorizeURL constructs the authorization URL to redirect to. The Trakt website will request
// permissions for your app and the user will have the opportunity to sign up for a new Trakt
// account or sign in with their existing account.
//
// Redirect URI is required, this is the URI which Trakt will redirect to after the authorization flow is
// complete. You can use "urn:ietf:wg:oauth:2.0:oob" for device authentication.
// The state is a random unique string which will be returned to the Redirect URI. This can be compared against
// to reduce the risk of "Man-In-The-Middle" attacks. See: https://en.wikipedia.org/wiki/Man-in-the-middle_attack
func (c *client) AuthorizeURL(params *trakt.AuthorizationURLParams) (string, error) {
	if params == nil {
		return "", errors.New("params cannot be nil")
	}
	uv, err := query.Values(wrappedAuthorizeURLParams{
		AuthorizationURLParams: *params,
		ClientID:               c.b.Key(),
		ResponseType:           "code",
	})

	if err != nil {
		return ``, err
	}

	return c.b.OAuthURL() + trakt.FormatURLPath(`/oauth/authorize?`) + uv.Encode(), nil
}

// genericTokenParameters generic parameters required when exchanging a code
// or refreshing an access token.
type genericTokenParameters struct {
	// ClientID this is the client id which is assigned to you when you set up an
	// application on Trakt. See: https://trakt.tv/oauth/applications/new to create a new application.
	ClientID string `json:"client_id" url:"-"`
	// GrantType which is the type of exchange we are performing. This will either be "authorization_code"
	// when exchanging a code, or "refresh_token" when refreshing a token.
	GrantType grantType `json:"grant_type" url:"-"`
}

// wrappedExchangeCodeParams a complete request to exchange a code for an
// access token.
type wrappedExchangeCodeParams struct {
	*trakt.ExchangeCodeParams
	genericTokenParameters
}

// ExchangeCode takes the authorization code parameter sent back to your redirect_uri to get an access_token.
// Save the access_token so your app can authenticate the user by sending the Authorization header.
// The access_token is valid for 3 months. Save and use the refresh_token to get a new access_token
// without asking the user to re-authenticate.
// This function requires the same RedirectURI which was sent in the initial request to get the code
// and also the client Secret which is also assigned to you when you create an application on Trakt.
func ExchangeCode(params *trakt.ExchangeCodeParams) (*trakt.Token, error) {
	return getC().ExchangeCode(params)
}

// ExchangeCode takes the authorization code parameter sent back to your redirect_uri to get an access_token.
// Save the access_token so your app can authenticate the user by sending the Authorization header.
// The access_token is valid for 3 months. Save and use the refresh_token to get a new access_token
// without asking the user to re-authenticate.
// This function requires the same RedirectURI which was sent in the initial request to get the code
// and also the client Secret which is also assigned to you when you create an application on Trakt.
func (c *client) ExchangeCode(params *trakt.ExchangeCodeParams) (*trakt.Token, error) {
	t := &trakt.Token{}
	p := &wrappedExchangeCodeParams{params, genericTokenParameters{c.b.Key(), authorizationCode}}
	err := c.b.Call(http.MethodPost, `/oauth/token`, p, t)
	return t, err
}

// wrappedExchangeCodeParams a complete request to generate a new access token
// from a refresh token.
type wrappedRefreshTokenParams struct {
	*trakt.RefreshTokenParams
	genericTokenParameters
}

// RefreshToken uses the Refresh Token to get a new Access Token without asking the user to re-authenticate.
// The Access Token is valid for 3 months before it needs to be refreshed again.
// This function requires the same RedirectURI which was sent in the initial request to get the code
// and also the client Secret which is also assigned to you when you create an application on Trakt.
func RefreshToken(params *trakt.RefreshTokenParams) (*trakt.Token, error) {
	return getC().RefreshToken(params)
}

// RefreshToken uses the Refresh Token to get a new Access Token without asking the user to re-authenticate.
// The Access Token is valid for 3 months before it needs to be refreshed again.
// This function requires the same RedirectURI which was sent in the initial request to get the code
// and also the client Secret which is also assigned to you when you create an application on Trakt.
func (c *client) RefreshToken(params *trakt.RefreshTokenParams) (*trakt.Token, error) {
	t := &trakt.Token{}
	p := &wrappedRefreshTokenParams{params, genericTokenParameters{c.b.Key(), refreshToken}}
	err := c.b.Call(http.MethodPost, `/oauth/token`, p, t)
	return t, err
}

// wrappedRevokeTokenParams complete request in order to revoke a generated access token.
type wrappedRevokeTokenParams struct {
	*trakt.RevokeTokenParams
	// ClientID this is the client id which is assigned to you when you set up an
	// application on Trakt. See: https://trakt.tv/oauth/applications/new to create a new application.
	ClientID string `json:"client_id" url:"-"`
}

// RevokeToken revokes an access token when a user signs out of their Trakt account in your app.
// This is not required, but might improve the user experience so the user doesn't have an
// unused app connection hanging around.
// This function requires the client Secret which is also assigned to you when you create an application on Trakt.
func RevokeToken(params *trakt.RevokeTokenParams) error {
	return getC().RevokeToken(params)
}

// RevokeToken revokes an access token when a user signs out of their Trakt account in your app.
// This is not required, but might improve the user experience so the user doesn't have an
// unused app connection hanging around.
// This function requires the client Secret which is also assigned to you when you create an application on Trakt.
func (c *client) RevokeToken(params *trakt.RevokeTokenParams) error {
	p := &wrappedRevokeTokenParams{params, c.b.Key()}
	return c.b.Call(http.MethodPost, `/oauth/revoke`, p, nil)
}
