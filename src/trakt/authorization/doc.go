// Package authorization handles allowing us to generate an access token to perform actions on behalf
// of an authenticated user.
//
// Authorization - OAuth.
//
// The API uses OAuth2. If you know what's up with OAuth2, grab your library and starting rolling.
// If you have access to a web browser (mobile app, desktop app, website), use standard OAuth. If you don't
// have web browser access (media center plugins, smart watches, smart TVs, command line scripts, system services),
// use Device authentication.
//
// Application Flow:
//
// 1. Redirect to request Trakt access. Using the /oauth/authorize method, construct then redirect to this URL.
// The Trakt website will request permissions for your app and the user will have the opportunity to sign up
// for a new Trakt account or sign in with their existing account.
//
// 2. Trakt redirects back to your site. If the user accepts your request, Trakt redirects back to your site with
// a temporary code in a code GET parameter as well as the state (if provided) in the previous step in a
// state parameter. If the states don’t match, the request has been created by a third party and the process
// should be aborted.
//
// 3. Exchange the code for an access token. If everything looks good in step 2, exchange the code for an access
// token using the /oauth/token method. Save the access_token so your app can authenticate the user by
// sending the Authorization header as indicated below or in any example code. The access_token is valid for
// 3 months. Save and use the refresh_token to get a new access_token without asking the user to re-authenticate.
//
// Authorization - Device.
//
// Device authentication is for apps and services with limited input or display capabilities.
// This include media center plugins, smart watches, smart TVs, command line scripts, and system services.
// Your app displays an alphanumeric code (typically 8 characters) to the user. They are then instructed to
// visit the verification URL on their computer or mobile device. After entering the code, the user will be
// prompted to grant permission for your app. After your app gets permissions, the device receives an access_token
// and works like standard OAuth from that point on.
//
// Application Flow:
//
// 1. Generate codes. Your app calls /oauth/device/code to generate new codes. Save this entire response
// for later use.Display the code. Display the user_code and instruct the user to visit the verification_url
// on their computer or mobile device.Poll for authorization.
//
// 2. Poll the /oauth/device/token method to see if the user successfully authorizes your app. Use the device_code
// and poll at the interval (in seconds) to check if the user has authorized your app. Check the docs
// below for the specific error codes you need to handle. Use expires_in to stop polling after that many seconds,
// and gracefully instruct the user to restart the process. It is important to poll at the correct interval
// and also stop polling when expired.
//
// 3. Successful authorization. When you receive a 200 success response, save the access_token so your app can
// authenticate the user in methods that require it. The access_token is valid for 3 months. Save and use
// the refresh_token to get a new access_token without asking the user to re-authenticate.
// It's normal OAuth from this point.
package authorization
