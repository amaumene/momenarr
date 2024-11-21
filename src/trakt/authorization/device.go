package authorization

import (
	"context"
	"net/http"
	"time"

	"github.com/jacklaaa89/trakt"
)

// client the authorization client which is used for requests.
type client struct{ b trakt.BaseClient }

// newDeviceCodeParams request structure to generate a new device code.
type newDeviceCodeParams struct {
	*trakt.BasicParams
	// ClientID this is the client id which is assigned to you when you set up an
	// application on Trakt. See: https://trakt.tv/oauth/applications/new to create a new application.
	ClientID string `json:"client_id" url:"-"`
}

// NewCode Generates new codes to start the device authentication process. The device_code and interval
// will be used later to poll for the access_token. The user_code and verification_url should be presented
// to the user as mentioned in the flow steps above.
func NewCode(params *trakt.BasicParams) (*trakt.DeviceCode, error) {
	return getC().NewCode(params)
}

// NewCode Generates new codes to start the device authentication process. The device_code and interval
// will be used later to poll for the access_token. The user_code and verification_url should be presented
// to the user as mentioned in the flow steps above.
func (c *client) NewCode(params *trakt.BasicParams) (*trakt.DeviceCode, error) {
	d := &trakt.DeviceCode{}
	p := &newDeviceCodeParams{params, c.b.Key()}
	err := c.b.Call(http.MethodPost, "/oauth/device/code", p, &d)
	return d, err
}

type wrappedPollCodeParams struct {
	*trakt.PollCodeParams
	ClientID string `json:"client_id" url:"-"`
}

// Code implements ErrorHandler interface.
// Device polling status codes mean different things. This allows us to specifically
// react to error codes for polling for a access token.
func (wrappedPollCodeParams) Code(statusCode int) trakt.ErrorCode {
	// handle known errors codes.
	switch statusCode {
	case http.StatusBadRequest:
		return trakt.ErrorCodePendingDeviceCode
	case http.StatusNotFound:
		return trakt.ErrorCodeInvalidDeviceCode
	case http.StatusConflict:
		return trakt.ErrorCodeDeviceCodeUsed
	case http.StatusGone:
		return trakt.ErrorCodeDeviceCodeExpired
	case http.StatusTeapot:
		return trakt.ErrorCodeDeviceCodeDenied
	}

	// resort to the default error handler if the code is not known.
	return trakt.DefaultErrorHandler.Code(statusCode)
}

// Poll uses the device_code to poll at the interval (in seconds) to check if the user has authorized you app.
// Use expires_in to stop polling after that many seconds, and gracefully instruct the user to restart the process.
// It is important to poll at the correct interval and also stop polling when expired. When you receive a 200
// success response, save the access_token so your app can authenticate the user in methods that require it.
// The access_token is valid for 3 months. Save and use the refresh_token to get a new access_token without
// asking the user to re-authenticate. The interval and expires_in fields are supplied from the DeviceCode retrieved
// when generating a new code.
//
// this function is blocking until either:
//   - we receive a 200 response with access token from the API.
//   - the supplied context is marked as done
//   - the supplied expires in duration exceeds its deadline
//   - any errors which deem that the token cannot be processed are returned from the API.
//
// if you require more control over when your app blocks for the result, use PollAsync which returns a
// channel that the result is pushed to.
func Poll(params *trakt.PollCodeParams) (*trakt.Token, error) {
	return getC().Poll(params)
}

// Poll uses the device_code to poll at the interval (in seconds) to check if the user has authorized you app.
// Use expires_in to stop polling after that many seconds, and gracefully instruct the user to restart the process.
// It is important to poll at the correct interval and also stop polling when expired. When you receive a 200
// success response, save the access_token so your app can authenticate the user in methods that require it.
// The access_token is valid for 3 months. Save and use the refresh_token to get a new access_token without
// asking the user to re-authenticate. The interval and expires_in fields are supplied from the DeviceCode retrieved
// when generating a new code.
//
// this function is blocking until either:
//   - we receive a 200 response with access token from the API.
//   - the supplied context is marked as done
//   - the supplied expires in duration exceeds its deadline
//   - any errors which deem that the token cannot be processed are returned from the API.
//
// if you require more control over when your app blocks for the result, use PollAsync which returns a
// channel that the result is pushed to.
func (c *client) Poll(params *trakt.PollCodeParams) (*trakt.Token, error) {
	r := <-c.PollAsync(params)
	return r.Token, r.Err
}

// Poll uses the device_code to poll at the interval (in seconds) to check if the user has authorized you app.
// Use expires_in to stop polling after that many seconds, and gracefully instruct the user to restart the process.
// It is important to poll at the correct interval and also stop polling when expired. When you receive a 200
// success response, save the access_token so your app can authenticate the user in methods that require it.
// The access_token is valid for 3 months. Save and use the refresh_token to get a new access_token without
// asking the user to re-authenticate. The interval and expires_in fields are supplied from the DeviceCode retrieved
// when generating a new code.
//
// This function does not block but instead returns a read-only channel which will have the result of polling
// for the result once it is available.
func PollAsync(params *trakt.PollCodeParams) <-chan *trakt.PollResult {
	return getC().PollAsync(params)
}

// Poll uses the device_code to poll at the interval (in seconds) to check if the user has authorized you app.
// Use expires_in to stop polling after that many seconds, and gracefully instruct the user to restart the process.
// It is important to poll at the correct interval and also stop polling when expired. When you receive a 200
// success response, save the access_token so your app can authenticate the user in methods that require it.
// The access_token is valid for 3 months. Save and use the refresh_token to get a new access_token without
// asking the user to re-authenticate. The interval and expires_in fields are supplied from the DeviceCode retrieved
// when generating a new code.
//
// This function does not block but instead returns a read-only channel which will have the result of polling
// for the result once it is available.
func (c *client) PollAsync(params *trakt.PollCodeParams) <-chan *trakt.PollResult {
	cCtx := params.Context
	if cCtx == nil {
		cCtx = context.Background()
	}

	ch := make(chan *trakt.PollResult, 1)

	go func() {
		ctx, cnl := context.WithTimeout(cCtx, params.ExpiresIn)
		defer cnl()

		// perform initial request.
		t, err := c.poll(withContext(ctx, params))
		if err == nil {
			ch <- &trakt.PollResult{Token: t}
			return
		}

		traktError, ok := err.(*trakt.Error)
		if !ok || !canContinuePolling(traktError.Code) {
			ch <- &trakt.PollResult{Err: err}
			return
		}

		tk := time.NewTicker(params.Interval)
		defer func() {
			tk.Stop()
			close(ch)
		}()

		for {
			select {
			case <-tk.C:
				// perform a new poll.
				t, err = c.poll(withContext(ctx, params))
				if err == nil {
					ch <- &trakt.PollResult{Token: t}
					return
				}

				// determine if we can continue to poll
				// there are certain error codes which mean we can
				// continue.
				traktError, ok := err.(*trakt.Error)
				if !ok || !canContinuePolling(traktError.Code) {
					ch <- &trakt.PollResult{Err: err}
					return
				}
			case <-ctx.Done():
				// the assigned context has been signalled
				// return an error.
				ch <- &trakt.PollResult{Err: ctx.Err()}
				return
			}
		}
	}()

	return ch
}

// poll performs a HTTP request to poll for the status of authorization on a device code.
// This function should be called on the interval defined in the DeviceCode when it was generated.
func (c *client) poll(params *trakt.PollCodeParams) (*trakt.Token, error) {
	t := &trakt.Token{}
	p := &wrappedPollCodeParams{params, c.b.Key()}
	err := c.b.Call(http.MethodPost, "/oauth/device/token", p, t)
	return t, err
}

// withContext makes a shallow copy of the supplied params and returns it with the
// supplied context attached.
func withContext(ctx context.Context, w *trakt.PollCodeParams) *trakt.PollCodeParams {
	cp := *w
	cp.Context = ctx
	return &cp
}

// canContinuePolling determines if the status code returned means that we can
// continue polling.
func canContinuePolling(e trakt.ErrorCode) bool { return e == trakt.ErrorCodePendingDeviceCode }

// getC returns a copy of a authorization client with the currently defined backend attached.
func getC() *client { return &client{trakt.NewClient()} }
