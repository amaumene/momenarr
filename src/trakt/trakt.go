// Package trakt provides bindings for all of Trakt API calls.
package trakt

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/go-querystring/query"
)

// APIVersion is the currently supported API version
const APIVersion string = "2"

const (
	// default URL endpoints for the production / staging APIs
	apiURL     = "https://api.trakt.tv"
	stagingURL = "https://api-staging.trakt.tv"
	// default URLs used for generating the authorization URL
	oAuthURL        string = "https://trakt.tv"
	stagingOAuthURL string = "https://staging.trakt.tv"

	// clientVersion is the binding version
	clientVersion = "0.1.0"

	// defaultHTTPTimeout is the default timeout on the http.Client used by the library.
	defaultHTTPTimeout = 80 * time.Second

	// maxNetworkRetriesDelay and minNetworkRetriesDelay defines sleep time in milliseconds between
	// tries to send HTTP request again after network failure.
	maxNetworkRetriesDelay = 5000 * time.Millisecond
	minNetworkRetriesDelay = 500 * time.Millisecond

	// applicationTypeJSON the required content-type used in all requests.
	applicationTypeJSON = "application/json"
	// requestIDHeader the header which contains the requestID.
	requestIDHeader = "X-Request-ID"
)

var (
	// productionConfig default configuration for a production environment.
	productionConfig = &BackendConfig{URL: apiURL, oAuthURL: oAuthURL}
	// stagingConfig default configuration for the staging environment.
	stagingConfig = &BackendConfig{URL: stagingURL, oAuthURL: stagingOAuthURL}

	// defaultCondition the default condition function, with never returns an error.
	defaultCondition Condition = func() error { return nil }
	// supportedBackends a thread-safe way of retrieving / updating the HTTP backends.
	supportedBackends backends
	// encodedUserAgent the user-agent to send with all requests to trakt.
	encodedUserAgent string
	// The default HTTP client used for communication with the API.
	httpClient = &http.Client{Timeout: defaultHTTPTimeout}

	// key is the Trakt API key used globally in the binding.
	Key string
)

type (
	// iteratorFunc is a function which is used to generate an iterator.
	iteratorFunc func(ListParamsContainer, queryFunc, bool) Iterator
	// Condition a function which can be set prior to attempting to retrieve a frame
	// to determine if the arguments are valid. If the response of this function returns
	// an error, then the error on the iterator is set to the error, and no more paging is performed.
	Condition func() error
)

// init performs initial initialisation. defaulting to a production configuration.
func init() {
	encodedUserAgent = "Trakt/" + APIVersion + " Go/" + clientVersion
	Production()
}

// Production sets the API to use the production environment.
func Production() { setBackend(getBackendWithConfig(productionConfig)) }

// Staging sets up the API to use the staging environment.
func Staging() { setBackend(getBackendWithConfig(stagingConfig)) }

// WithConfig sets up the API with the supplied config.
func WithConfig(bk *BackendConfig) { setBackend(getBackendWithConfig(bk)) }

// NewClient generates a new client which all other clients should inherit from.
func NewClient() BaseClient { return &baseClient{B: getBackend(), key: Key} }

// BackendConfig is used to configure a new Trakt backend.
type BackendConfig struct {
	// client is an HTTP client instance to use when making API requests.
	//
	// If left unset, it'll be set to a default HTTP client for the package.
	HTTPClient *http.Client

	// leveledLogger is the logger that the backend will use to log errors,
	// warnings, and informational messages.
	LeveledLogger LeveledLoggerInterface

	// maxNetworkRetries sets maximum number of times that the library will
	// retry requests that appear to have failed due to an intermittent
	// problem.
	//
	// Defaults to 0.
	MaxNetworkRetries int

	// URL is the base URL to use for API paths.
	URL string

	// oAuthURL is the base URL to use for OAuth paths.
	// this cannot be overridden.
	oAuthURL string
}

// headerUnmarshaller interface which is used to inform the unmarshaller
// that we need to unmarshal this type using the response headers.
type headerUnmarshaller interface {
	// unmarshalHeaders allows us to unmarshal using data supplied in the
	// response errors.
	unmarshalHeaders(h http.Header) error
}

// backend is an interface for making calls against a trakt service.
type backend interface {
	// call performs a call to path using the defined HTTP method and unmarshalling the result into v.
	call(method, path, key string, params ParamsContainer, v interface{}) error
	// callWithFrame performs a call using the defined HTTP method and unmarshalling into the supplied iterationFrame.
	callWithFrame(method, path, key string, params ParamsContainer, v iterationFrame) error
	// oAuthURL returns the
	oAuthURL() string
}

// BaseClient the base implementation of a client.
// this gives access to methods all clients will use without giving the client access
// to any of the underlined HTTP handling code.
type BaseClient interface {
	// NewIterator creates a new iterator which paginates through a list of results until the
	// end or the defined limit. the iterator uses the supplied method and path to create the URL
	// to use.
	NewIterator(method, path string, p ListParamsContainer) Iterator

	// NewIteratorWithCondition creates a new iterator which paginates through a list of results
	// but with a condition which is invokes before every frame is queried to see if the params
	// are still valid.
	NewIteratorWithCondition(method, path string, p ListParamsContainer, cnd Condition) Iterator

	// NewIterator creates a new iterator which paginates through a list of results until the
	// end or the defined limit. the iterator uses the supplied method and path to create the URL
	// to use.
	// A simulated iterator is an iterator instance which implements the standard Iterator interface
	// but only actually loads for the first page, because there are API endpoints which return a list
	// but dont require pagination, this allows us to have a standard interface for all lists.
	NewSimulatedIterator(method, path string, p ListParamsContainer) Iterator

	// NewIteratorWithCondition creates a new iterator which paginates through a list of results
	// but with a condition which is invokes before every frame is queried to see if the params
	// are still valid.
	// A simulated iterator is an iterator instance which implements the standard Iterator interface
	// but only actually loads for the first page, because there are API endpoints which return a list
	// but dont require pagination, this allows us to have a standard interface for all lists.
	NewSimulatedIteratorWithCondition(method, path string, p ListParamsContainer, cnd Condition) Iterator

	// Call performs a simple HTTP call and unmarshals the result into v.
	Call(method, path string, params ParamsContainer, v interface{}) error

	// key returns the assigned clientID, this is mainly used for authorization.
	Key() string

	// OAuthURL returns the OAuthURL assigned to the config.
	OAuthURL() string
}

// backends are the currently supported endpoints.
type backends struct {
	sync.RWMutex
	API backend
}

// baseClient a base client which gives us default functionality to parent client implementations.
type baseClient struct {
	// B the backend to query, this is an interface by design
	// it allows us to override the underlined HTTP backend for tests etc
	B backend
	// key the API (client_id) to use in requests, this is inherited from the var key
	key string
}

// oAuthURL returns the defined oAuthURL for the configuration.
func (b *baseClient) OAuthURL() string { return b.B.oAuthURL() }

// NewIterator implements BaseClient interface.
func (b *baseClient) NewIterator(method, path string, p ListParamsContainer) Iterator {
	return b.newIteratorWithReceiver(newIterator, method, path, p, defaultCondition, false)
}

// NewIteratorWithCondition implements BaseClient interface.
func (b *baseClient) NewIteratorWithCondition(method, path string, p ListParamsContainer, cnd Condition) Iterator {
	return b.newIteratorWithReceiver(newIterator, method, path, p, cnd, false)
}

// NewSimulatedIterator implements BaseClient interface.
func (b *baseClient) NewSimulatedIterator(method, path string, p ListParamsContainer) Iterator {
	return b.newIteratorWithReceiver(newSimulatedIterator, method, path, p, defaultCondition, false)
}

// NewIteratorWithCondition implements BaseClient interface.
func (b *baseClient) NewSimulatedIteratorWithCondition(method, path string, p ListParamsContainer, cnd Condition) Iterator {
	return b.newIteratorWithReceiver(newSimulatedIterator, method, path, p, cnd, false)
}

// Key implements BaseClient interface.
func (b *baseClient) Key() string { return b.key }

// newIteratorWithReceiver helper function which performs all of the boilerplate code to generate an
// iterator. Using this method over generating an iterator is far more efficient, as we can re-use the rcv pointer
// for each frame, rather than allocate a new rcv for each frame processed.
func (b *baseClient) newIteratorWithReceiver(
	generator iteratorFunc, method, path string, p ListParamsContainer, cnd Condition, lazyLoad bool,
) Iterator {
	return generator(p, func(i ListParamsContainer) (iterationFrame, error) {
		f := newEmptyFrame()
		if cErr := cnd(); cErr != nil {
			return f, cErr
		}

		err := b.B.callWithFrame(method, path, b.Key(), i, f)
		return f, err
	}, lazyLoad)
}

// Call helper function function which calls the underlined backend providing the assigned key.
func (b *baseClient) Call(method, path string, params ParamsContainer, v interface{}) error {
	return b.B.call(method, path, b.Key(), params, v)
}

// backendImplementation is the internal implementation for making HTTP calls
// to Trakt.
type backendImplementation struct {
	URL               string
	authURL           string
	client            *http.Client
	leveledLogger     LeveledLoggerInterface
	maxNetworkRetries int

	// networkRetriesSleep indicates whether the backend should use the normal
	// sleep between retries.
	//
	// See also SetNetworkRetriesSleep.
	networkRetriesSleep bool
}

// oAuthURL returns the oAuthURL for the backend.
func (s *backendImplementation) oAuthURL() string { return s.authURL }

// callWithFrame called for pagination type functions where we are typically querying for a single frame / segment
// in the entire set. This puts a higher level of abstraction above that of the basic call function.
func (s *backendImplementation) callWithFrame(method, path, key string, params ParamsContainer, v iterationFrame) error {
	if v == nil {
		return &Error{Resource: path, Code: ErrorCodeEmptyFrameData, Body: "frame data is required"}
	}

	return s.callRaw(method, path, key, params, v.rcv(), v)
}

// call implements the backend interface. performs a call to the trakt API.
func (s *backendImplementation) call(method, path, key string, params ParamsContainer, v interface{}) error {
	return s.callRaw(method, path, key, params, v, nil)
}

// callRaw executes a HTTP request to the trakt API by generating the path, body and executing the request and
// unmarshalling the response into v.
func (s *backendImplementation) callRaw(method, path, key string, params ParamsContainer, v, h interface{}) error {
	rv := reflect.ValueOf(params)
	if !rv.IsValid() || rv.IsNil() {
		params = &BasicParams{}
	}

	var body []byte
	if isHTTPWriteMethod(method) {
		var encodeErr error
		// perform a JSON encode of the q as the body.
		// the tags defined will dictate what is set in the body JSON
		body, encodeErr = json.Marshal(params)
		if encodeErr != nil {
			return encodeErr
		}
	} else {
		uv, err := query.Values(params)
		if err != nil {
			return err
		}

		path += `?` + uv.Encode()
	}

	bodyBuffer := bytes.NewBuffer(body)

	req, err := s.newRequest(method, path, key, applicationTypeJSON, params)
	if err != nil {
		return err
	}

	// perform the request.
	if err := s.do(req, bodyBuffer, params, v, h); err != nil {
		return err
	}

	return nil
}

// newRequest is used by call / callWithFrame to generate an http.Request. It handles encoding
// parameters and attaching the appropriate headers.
func (s *backendImplementation) newRequest(method, path, key, contentType string, params ParamsContainer) (*http.Request, error) {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	path = s.URL + path
	req, err := http.NewRequest(method, path, nil)
	if err != nil {
		s.leveledLogger.Errorf("Cannot create request: %v", err)
		return nil, err
	}

	req.Header.Add("Content-Type", contentType)
	req.Header.Add("User-Agent", encodedUserAgent)

	// add trakt specific headers.
	req.Header.Add("trakt-api-version", APIVersion)
	req.Header.Add("trakt-api-key", key)

	// add oauth token if supplied.
	token := params.oauth()

	if token != "" {
		authorization := "Bearer " + token
		req.Header.Add("Authorization", authorization)
	}

	req = req.WithContext(params.context())

	for k, v := range params.headers() {
		for _, line := range v {
			// Use Set to override the default value possibly set before
			req.Header.Set(k, line)
		}
	}

	return req, nil
}

// do is used by call / callWithFrame to execute an API request and parse the response. It uses
// the backend's HTTP client to execute the request and unmarshals the response
// into v. It also handles unmarshaling errors returned by the API.
func (s *backendImplementation) do(req *http.Request, body *bytes.Buffer, params ParamsContainer, v, h interface{}) error {
	s.leveledLogger.Infof("Requesting %v %v%v\n", req.Method, req.URL.Host, req.URL.Path)

	var res *http.Response
	var err error
	var requestDuration time.Duration
	var resBody []byte
	for retry := 0; ; {
		start := time.Now()

		if body != nil {
			reader := bytes.NewReader(body.Bytes())

			req.Body = nopReadCloser{reader}
			req.GetBody = func() (io.ReadCloser, error) {
				reader := bytes.NewReader(body.Bytes())
				return nopReadCloser{reader}, nil
			}
		}

		res, err = s.client.Do(req)

		requestDuration = time.Since(start)
		s.leveledLogger.Infof("Request completed in %v (retry: %v)", requestDuration, retry)

		if err == nil {
			resBody, err = ioutil.ReadAll(res.Body)
			res.Body.Close()
		}

		if err != nil {
			s.leveledLogger.Errorf("Request failed with error: %v", err)
		} else if res.StatusCode >= 400 {
			err = s.responseToError(res, resBody, params)
		}

		// If the response was okay, or an error that shouldn't be retried,
		// we're done, and it's safe to leave the retry loop.
		if !s.shouldRetry(err, req, res, retry) {
			break
		}

		sleepDuration := s.sleepTime(retry)
		retry++

		s.leveledLogger.Warnf("Initiating retry %v for request %v %v%v after sleeping %v",
			retry, req.Method, req.URL.Host, req.URL.Path, sleepDuration)

		time.Sleep(sleepDuration)
	}

	if err != nil {
		return err
	}

	s.leveledLogger.Debugf("Response: %s\n", string(resBody))

	if v != nil {
		return s.unmarshalJSONVerbose(res, resBody, v, h)
	}

	return nil
}

// responseToError converts a trakt HTTP status code response to an error.
func (s *backendImplementation) responseToError(res *http.Response, resBody []byte, p ParamsContainer) error {
	var errorHandler = DefaultErrorHandler
	switch e := p.(type) {
	case ErrorHandler:
		errorHandler = e
	}

	return &Error{
		HTTPStatusCode: res.StatusCode,
		RequestID:      res.Header.Get(requestIDHeader),
		Body:           string(resBody),
		Resource:       res.Request.URL.Path,
		Code:           errorHandler.Code(res.StatusCode),
	}
}

// unmarshalJSONVerbose unmarshals JSON, but in case of a failure logs and
// produces a more descriptive error.
// rcv represents the receiver for the JSON response.
// con represents the container for the receiver. This can be nil or an iterator
// wrapper if we are unmarshalling a single frame from a iteration result.
func (s *backendImplementation) unmarshalJSONVerbose(res *http.Response, body []byte, rcv, con interface{}) error {
	err := json.Unmarshal(body, rcv)
	if err != nil {
		bodySample := string(body)
		if len(bodySample) > 500 {
			bodySample = bodySample[0:500] + " ..."
		}

		newErr := s.responseToError(res, []byte(strings.Replace(bodySample, "\n", "\\n", -1)), nil)
		newErr.(*Error).Code = ErrorCodeEncodingError

		s.leveledLogger.Errorf("%s", newErr)
		return newErr
	}

	// perform some custom unmarshal's if required.
	// we always want to perform JSON unmarshalling to.
	// this is just to grab additional information that is
	// not just provided in the response body.
	err = unmarshalHeaders(res, con)
	if err != nil {
		s.leveledLogger.Errorf("%s", err)
		return err
	}

	return err
}

// unmarshalHeaders quick function to unmarshal the HTTP response headers
// into the receiver if it implements the required interface.
func unmarshalHeaders(res *http.Response, i interface{}) error {
	switch m := i.(type) {
	case headerUnmarshaller:
		return m.unmarshalHeaders(res.Header)
	}
	return nil
}

// Checks if an error is a problem that we should retry on. This includes both
// socket errors that may represent an intermittent problem and some special
// HTTP statuses.
func (s *backendImplementation) shouldRetry(err error, req *http.Request, resp *http.Response, numRetries int) bool {
	if numRetries >= s.maxNetworkRetries {
		return false
	}

	traktErr, _ := err.(*Error)

	if traktErr == nil && err != nil {
		return true
	}

	// 409 Conflict
	if resp.StatusCode == http.StatusConflict {
		return true
	}

	// 429 Too Many Requests
	//
	// There are a few different problems that can lead to a 429. The most
	// common is rate limiting, on which we *don't* want to retry because
	// that'd likely contribute to more contention problems. However, some 429s
	// are lock timeouts, which is when a request conflicted with another
	// request or an internal process on some particular object. These 429s are
	// safe to retry.
	if resp.StatusCode == http.StatusTooManyRequests {
		return false
	}

	// 500 Internal Server Error
	if resp.StatusCode >= http.StatusInternalServerError {
		return true
	}

	// 503 Service Unavailable
	if resp.StatusCode == http.StatusServiceUnavailable {
		return true
	}

	// 503 Gateway Timeout
	if resp.StatusCode == http.StatusGatewayTimeout {
		return true
	}

	// if we have a cloudflare error (which dont use standard HTTP status codes)
	if isCloudflareError(resp.StatusCode) {
		return true
	}

	return false
}

// sleepTime calculates sleeping/delay time in milliseconds between failure and a new one request.
func (s *backendImplementation) sleepTime(numRetries int) time.Duration {
	// We disable sleeping in some cases for tests.
	if !s.networkRetriesSleep {
		return 0 * time.Second
	}

	// Apply exponential backoff with minNetworkRetriesDelay on the
	// number of num_retries so far as inputs.
	delay := minNetworkRetriesDelay + minNetworkRetriesDelay*time.Duration(numRetries*numRetries)

	// Do not allow the number to exceed maxNetworkRetriesDelay.
	if delay > maxNetworkRetriesDelay {
		delay = maxNetworkRetriesDelay
	}

	// Apply some jitter by randomizing the value in the range of 75%-100%.
	jitter := rand.Int63n(int64(delay / 4))
	delay -= time.Duration(jitter)

	// But never sleep less than the base sleep seconds.
	if delay < minNetworkRetriesDelay {
		delay = minNetworkRetriesDelay
	}

	return delay
}

// FormatURLPath takes a format string (of the kind used in the fmt package)
// representing a URL path with a number of parameters that belong in the path
// and returns a formatted string.
//
// we perform a quick type comparison to handle some well-known types
// including a SearchID or a fmt.Stringer or string or any primitive int/uint type, it any of the
// parameters are not of these types, then we ignore the value
// as it should only be of of these three types.
// this would cause come very noticeable formatting errors.
func FormatURLPath(format string, params ...interface{}) string {
	// Convert parameters to interface{} and URL-escape them
	untypedParams := make([]interface{}, len(params))
	var i int
	for _, param := range params {
		formatted, err := formatInterface(param)
		if err != nil {
			continue
		}

		untypedParams[i] = interface{}(url.QueryEscape(formatted))
		i++
	}

	return fmt.Sprintf(format, untypedParams[:i]...)
}

// formatInterface attempts to format an interface into a string.
func formatInterface(i interface{}) (string, error) {
	if i == nil {
		return "", nil
	}

	v := reflect.ValueOf(i)
	if v.Kind() == reflect.Ptr {
		return formatInterface(v.Elem().Interface())
	}

	// handle primitive types.
	switch v.Kind() {
	case reflect.Array, reflect.Slice:
		return formatSlice(v.Interface()), nil
	case reflect.Int, reflect.Int8, reflect.Int16,
		reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16,
		reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(v.Float(), 'g', -1, 64), nil
	case reflect.Complex64, reflect.Complex128:
		c := v.Complex()
		return strconv.FormatFloat(real(c), 'g', -1, 64) +
			"+" + strconv.FormatFloat(imag(c), 'g', -1, 64) + "i", nil
	case reflect.String:
		return v.String(), nil
	case reflect.Bool:
		if v.Bool() {
			return "true", nil
		}
		return "false", nil
	}

	// handle special types.
	switch p := i.(type) {
	case fmt.Stringer:
		return p.String(), nil
	case SearchID:
		return p.id(), nil
	default:
		return "", errors.New("invalid interface type")
	}
}

// formatSlice attempts to format a slice into a comma separated
// joined string.
func formatSlice(slice interface{}) string {
	if slice == nil {
		return ""
	}

	v := reflect.ValueOf(slice)

	switch v.Kind() {
	case reflect.Ptr:
		return formatSlice(v.Elem().Interface())
	case reflect.Slice, reflect.Array:
		break
	default:
		return ""
	}

	stringSlice := make([]string, v.Len())
	var idx int
	for i := 0; i < v.Len(); i++ {
		f := v.Index(i)
		str, err := formatInterface(f.Interface())
		if err != nil {
			continue
		}
		stringSlice[idx] = str
		idx++
	}

	return strings.Join(stringSlice[:idx], ",")
}

// getBackend retrieves the API backend.
func getBackend() backend {
	supportedBackends.RLock()
	defer supportedBackends.RUnlock()

	return supportedBackends.API
}

// getBackendWithConfig is the same as getBackend except that it can be given a
// configuration struct that will configure certain aspects of the backend
// that's return.
func getBackendWithConfig(config *BackendConfig) backend {
	if config.HTTPClient == nil {
		config.HTTPClient = httpClient
	}

	if config.LeveledLogger == nil {
		config.LeveledLogger = DefaultLeveledLogger
	}

	if config.URL == "" {
		config.URL = apiURL
	}

	if config.oAuthURL == "" {
		config.oAuthURL = oAuthURL
	}

	config.URL = normalizeURL(config.URL)
	config.oAuthURL = normalizeURL(config.oAuthURL)

	return newBackendImplementation(config)
}

// setBackend sets the backend used in the binding.
func setBackend(b backend) {
	supportedBackends.Lock()
	defer supportedBackends.Unlock()
	supportedBackends.API = b
}

// nopReadCloser's sole purpose is to give us a way to turn an `io.Reader` into
// an `io.ReadCloser` by adding a no-op implementation of the `Closer`
// interface. We need this because `http.Request`'s `Body` takes an
// `io.ReadCloser` instead of a `io.Reader`.
type nopReadCloser struct {
	io.Reader
}

// Close implements io.Reader interface.
func (nopReadCloser) Close() error { return nil }

// newBackendImplementation returns a new backend based off a given type and
// fully initialized BackendConfig struct.
//
// The vast majority of the time you should be calling getBackendWithConfig
// instead of this function.
func newBackendImplementation(config *BackendConfig) backend {
	return &backendImplementation{
		client:              config.HTTPClient,
		leveledLogger:       config.LeveledLogger,
		maxNetworkRetries:   config.MaxNetworkRetries,
		URL:                 config.URL,
		networkRetriesSleep: true,
	}
}

// isHTTPWriteMethod determines if the HTTP supplied is a mutation type.
func isHTTPWriteMethod(method string) bool {
	return method == http.MethodPost || method == http.MethodPut ||
		method == http.MethodPatch || method == http.MethodDelete
}

// isCloudflareError helper function to determine if a status code represents
// an error from cloudflare.
func isCloudflareError(code int) bool {
	return code == 520 || code == 521 || code == 522
}

// normalizeURL function which takes a URL string and normalizes it.
func normalizeURL(url string) string {
	// All paths include a leading slash, so to keep logs pretty, trim a
	// trailing slash on the URL.
	url = strings.TrimSuffix(url, "/")

	return url
}
